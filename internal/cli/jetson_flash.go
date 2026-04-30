package cli

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	setup "github.com/anthropic-lab/internkim/internal/provisioning/steps"
)

const defaultJetsonOrinNanoImageURL = "https://developer.nvidia.com/downloads/embedded/L4T/r36_Release_v4.4/jp62-r1-orin-nano-sd-card-image.zip"
const jetsonOrinNanoSDKPageURL = "https://developer.nvidia.com/embedded/jetpack-sdk-622"
const minimumJetsonFlashCacheBytes = 45 * 1024 * 1024 * 1024
const jetsonFirstbootScriptPath = "/usr/local/bin/internkim-jetson-firstboot.sh"
const jetsonFirstbootServicePath = "/etc/systemd/system/internkim-jetson-firstboot.service"
const jetsonAutologinOverridePath = "/etc/systemd/system/getty@tty1.service.d/override.conf"
const jetsonLegacyNetworkManagerConnectionPath = "/etc/NetworkManager/system-connections/internkim-wifi.nmconnection"
const jetsonWiFiSelectorScriptPath = "/usr/local/bin/internkim-wifi-select"

type flashImage struct {
	path string
}

func runFlash() {
	lang := "ko"
	if containsArg("--en") {
		lang = "en"
	}
	messenger := newMsg(lang)
	boardType := argString("--board", "")
	if boardType == "" {
		boardType = setup.BoardJetsonOrinNano
	}
	if boardType != setup.BoardJetsonOrinNano {
		fatal("flash currently supports --board jetson-orin-nano")
	}
	if containsArg("--fix-oem-user") {
		if err := runJetsonOEMUserFix(messenger); err != nil {
			fatal(err.Error())
		}
		return
	}
	if err := runJetsonFlash(messenger); err != nil {
		fatal(err.Error())
	}
}

func runJetsonOEMUserFix(messenger *msg) error {
	stateDirectory := internkimHomeDir()
	scriptDirectory, _ := os.Getwd()
	disk := argString("--disk", "")
	if disk == "" {
		disk = detectSDCard()
	}
	if disk == "" {
		return errors.New(messenger.t("SD 카드를 찾을 수 없습니다. Jetson에서 SD를 빼서 맥에 꽂거나 --disk /dev/diskN 으로 지정하세요.", "SD card not found. Remove it from the Jetson, insert it into the Mac, or pass --disk /dev/diskN."))
	}

	username := firstNonEmptyString(argString("--jetson-user", ""), jetsonDefaultUser)
	password := firstNonEmptyString(argString("--jetson-password", ""), jetsonDefaultPassword)
	wifiProfiles, errorValue := resolveWiFiProfiles(messenger, stateDirectory, filepath.Join(scriptDirectory, "bin", "get-ssid"))
	if errorValue != nil {
		return errorValue
	}
	fmt.Printf("=== Intern Kim Jetson OEM User Fix ===\n\n")
	fmt.Printf("  %s: %s\n", messenger.t("대상 디스크", "Target disk"), disk)
	fmt.Printf("  %s: %s\n", messenger.t("사용자", "User"), username)
	if len(wifiProfiles) > 0 {
		fmt.Printf("  Wi-Fi profiles: %d\n", len(wifiProfiles))
	}
	if !containsArg("--yes") && !promptYN(messenger.t(
		"SD 카드의 Linux rootfs에 기본 사용자를 주입합니다. 계속하시겠습니까?",
		"This will inject a default user into the SD card Linux rootfs. Continue?",
	)) {
		return errors.New(messenger.t("취소됨.", "Cancelled."))
	}
	if errorValue := ensureSudoReady(); errorValue != nil {
		return errorValue
	}

	partitionDevice, errorValue := resolveJetsonRootPartition(disk)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := repairJetsonRootFilesystem(partitionDevice); errorValue != nil {
		return errorValue
	}
	rootPatch := jetsonRootPatch{
		partitionDevice: partitionDevice,
		username:        username,
		password:        password,
		publicKey:       getLocalSSHPubKey(),
		wifiProfiles:    wifiProfiles,
	}
	if errorValue := applyJetsonRootPatch(rootPatch); errorValue != nil {
		return errorValue
	}
	if errorValue := verifyJetsonRootPatch(rootPatch); errorValue != nil {
		return errorValue
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("  %s\n", messenger.t("Jetson 기본 사용자 주입 완료", "Jetson default user injected"))
	fmt.Println("========================================")
	fmt.Println()
	fmt.Printf("  user: %s\n", username)
	fmt.Printf("  password: %s\n", password)
	fmt.Println(messenger.t("  SD 카드를 Jetson에 다시 꽂고 부팅하세요.", "  Put the SD card back into the Jetson and boot it."))
	return nil
}

func ensureSudoReady() error {
	command := exec.Command("sudo", "-v")
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if errorValue := command.Run(); errorValue != nil {
		return fmt.Errorf("sudo authentication failed: %w", errorValue)
	}
	return nil
}

func resolveJetsonFlashWiFi(messenger *msg, stateDirectory string, getSSIDPath string) (string, string, error) {
	wifiProfiles, errorValue := resolveWiFiProfiles(messenger, stateDirectory, getSSIDPath)
	if errorValue != nil {
		return "", "", errorValue
	}
	if len(wifiProfiles) == 0 {
		return "", "", nil
	}
	return wifiProfiles[0].SSID, wifiProfiles[0].Password, nil
}

func runJetsonFlash(messenger *msg) error {
	cacheDirectory := jetsonFlashCacheDirectory()
	if errorValue := os.MkdirAll(cacheDirectory, 0o755); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureJetsonFlashCacheSpace(cacheDirectory); errorValue != nil {
		return errorValue
	}

	disk := argString("--disk", "")
	if disk == "" {
		disk = detectSDCard()
	}
	if disk == "" {
		return errors.New(messenger.t("SD 카드를 찾을 수 없습니다. --disk /dev/diskN 으로 지정할 수도 있습니다.", "SD card not found. You can pass --disk /dev/diskN."))
	}

	image, errorValue := resolveJetsonFlashImage(cacheDirectory)
	if errorValue != nil {
		return errorValue
	}

	fmt.Printf("=== Intern Kim Flash (%s) ===\n\n", setup.BoardJetsonOrinNano)
	fmt.Printf("  %s: %s\n", messenger.t("대상 디스크", "Target disk"), disk)
	fmt.Printf("  %s: %s\n", messenger.t("이미지", "Image"), image.path)
	if !containsArg("--yes") && !promptYN(messenger.t(
		disk+" 의 모든 데이터가 삭제됩니다. 계속하시겠습니까?",
		"All data on "+disk+" will be erased. Continue?",
	)) {
		return errors.New(messenger.t("취소됨.", "Cancelled."))
	}

	if errorValue := writeJetsonImageToDisk(image.path, disk); errorValue != nil {
		return errorValue
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("  %s\n", messenger.t("Jetson SD 카드 준비 완료", "Jetson SD card ready"))
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println(messenger.t(
		"  1. SD 카드를 Jetson에 삽입\n  2. ./internkim flash --fix-oem-user 로 기본 사용자/Wi-Fi 주입\n  3. Jetson 부팅\n  4. ./internkim setup",
		"  1. Insert the SD card into the Jetson\n  2. Inject the default user/Wi-Fi with ./internkim flash --fix-oem-user\n  3. Boot the Jetson\n  4. ./internkim setup",
	))
	return nil
}

func resolveJetsonFlashImage(cacheDirectory string) (flashImage, error) {
	localImagePath := strings.TrimSpace(argString("--image", ""))
	if localImagePath != "" {
		imagePath, errorValue := prepareJetsonImage(localImagePath, cacheDirectory)
		return flashImage{path: imagePath}, errorValue
	}

	imageURL := strings.TrimSpace(argString("--image-url", ""))
	if imageURL == "" {
		imageURL = strings.TrimSpace(os.Getenv("INTERNKIM_JETSON_IMAGE_URL"))
	}
	if imageURL == "" {
		imageURL = defaultJetsonOrinNanoImageURL
	}

	archivePath := filepath.Join(cacheDirectory, filepath.Base(imageURL))
	if isInvalidJetsonImageCache(archivePath) {
		_ = os.Remove(archivePath)
	}
	if _, errorValue := os.Stat(archivePath); os.IsNotExist(errorValue) {
		fmt.Printf("  JetPack image download: %s\n", imageURL)
		partialArchivePath := archivePath + ".partial"
		_ = os.Remove(partialArchivePath)
		command := exec.Command("curl", "-fL", "--progress-bar", "-o", partialArchivePath, imageURL)
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if errorValue := command.Run(); errorValue != nil {
			_ = os.Remove(partialArchivePath)
			return flashImage{}, fmt.Errorf("download Jetson image failed. This is usually local disk space or write failure, not a network timeout. Free at least 45GB or pass --cache-dir <external-disk-path>. You can also download it from %s and rerun with --image <zip-or-img>, or pass --image-url: %w", jetsonOrinNanoSDKPageURL, errorValue)
		}
		if errorValue := os.Rename(partialArchivePath, archivePath); errorValue != nil {
			_ = os.Remove(partialArchivePath)
			return flashImage{}, errorValue
		}
	}

	imagePath, errorValue := prepareJetsonImage(archivePath, cacheDirectory)
	return flashImage{path: imagePath}, errorValue
}

func isInvalidJetsonImageCache(archivePath string) bool {
	fileInfo, errorValue := os.Stat(archivePath)
	if os.IsNotExist(errorValue) {
		return false
	}
	if errorValue != nil || fileInfo.IsDir() || fileInfo.Size() == 0 {
		return true
	}
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		reader, errorValue := zip.OpenReader(archivePath)
		if errorValue != nil {
			return true
		}
		_ = reader.Close()
	}
	return false
}

func jetsonFlashCacheDirectory() string {
	if cacheDirectory := strings.TrimSpace(argString("--cache-dir", "")); cacheDirectory != "" {
		return filepath.Clean(cacheDirectory)
	}
	if cacheDirectory := strings.TrimSpace(os.Getenv("INTERNKIM_FLASH_CACHE_DIR")); cacheDirectory != "" {
		return filepath.Clean(cacheDirectory)
	}
	return filepath.Join(internkimHomeDir(), "cache", "jetson")
}

func ensureJetsonFlashCacheSpace(cacheDirectory string) error {
	availableBytes, errorValue := availableDiskBytes(cacheDirectory)
	if errorValue != nil {
		return errorValue
	}
	if availableBytes >= minimumJetsonFlashCacheBytes {
		return nil
	}
	return fmt.Errorf("Jetson flash cache needs at least 45GB free for the ZIP and extracted sd-blob.img; %s has %s free. Free disk space or pass --cache-dir <external-disk-path>", cacheDirectory, humanBytes(availableBytes))
}

func availableDiskBytes(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if errorValue := syscall.Statfs(path, &stat); errorValue != nil {
		return 0, errorValue
	}
	return uint64(stat.Bavail) * uint64(stat.Bsize), nil
}

func humanBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%dB", value)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	floatValue := float64(value)
	for _, unitName := range units {
		floatValue = floatValue / unit
		if floatValue < unit {
			return fmt.Sprintf("%.1f%s", floatValue, unitName)
		}
	}
	return fmt.Sprintf("%.1fPiB", floatValue/unit)
}

func prepareJetsonImage(sourcePath string, cacheDirectory string) (string, error) {
	cleanSourcePath := filepath.Clean(sourcePath)
	lowerPath := strings.ToLower(cleanSourcePath)
	switch {
	case strings.HasSuffix(lowerPath, ".img"):
		return cleanSourcePath, nil
	case strings.HasSuffix(lowerPath, ".zip"):
		return extractJetsonImageZip(cleanSourcePath, filepath.Join(cacheDirectory, strings.TrimSuffix(filepath.Base(cleanSourcePath), filepath.Ext(cleanSourcePath))))
	default:
		return "", fmt.Errorf("unsupported Jetson image format: %s", cleanSourcePath)
	}
}

func extractJetsonImageZip(zipPath string, outputDirectory string) (string, error) {
	imagePath := filepath.Join(outputDirectory, "sd-blob.img")
	if fileInfo, errorValue := os.Stat(imagePath); errorValue == nil && !fileInfo.IsDir() && fileInfo.Size() > 0 {
		return imagePath, nil
	}

	reader, errorValue := zip.OpenReader(zipPath)
	if errorValue != nil {
		return "", errorValue
	}
	defer reader.Close()

	for _, file := range reader.File {
		if file.FileInfo().IsDir() || !strings.HasSuffix(strings.ToLower(file.Name), ".img") {
			continue
		}
		if errorValue := os.MkdirAll(outputDirectory, 0o755); errorValue != nil {
			return "", errorValue
		}
		if errorValue := extractZipFile(file, imagePath); errorValue != nil {
			return "", errorValue
		}
		return imagePath, nil
	}
	return "", fmt.Errorf("no .img file found in %s", zipPath)
}

func extractZipFile(file *zip.File, targetPath string) error {
	source, errorValue := file.Open()
	if errorValue != nil {
		return errorValue
	}
	defer source.Close()

	target, errorValue := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if errorValue != nil {
		return errorValue
	}
	defer target.Close()

	_, errorValue = io.Copy(target, source)
	return errorValue
}

func writeJetsonImageToDisk(imagePath string, disk string) error {
	if _, errorValue := os.Stat(imagePath); errorValue != nil {
		return errorValue
	}
	if !strings.HasPrefix(disk, "/dev/disk") {
		return fmt.Errorf("refusing to flash non-macOS disk path: %s", disk)
	}

	_ = exec.Command("diskutil", "unmountDisk", disk).Run()
	rawDisk := strings.Replace(disk, "/dev/disk", "/dev/rdisk", 1)
	command := exec.Command("sudo", "dd", "if="+imagePath, "of="+rawDisk, "bs=4m", "status=progress")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if errorValue := command.Run(); errorValue != nil {
		return fmt.Errorf("write Jetson image failed: %w", errorValue)
	}
	_ = exec.Command("sync").Run()
	time.Sleep(time.Second)
	_ = exec.Command("diskutil", "unmountDisk", disk).Run()
	return nil
}

func resolveJetsonRootPartition(disk string) (string, error) {
	if rootPartition := strings.TrimSpace(argString("--root-partition", "")); rootPartition != "" {
		return rootPartition, nil
	}
	return findJetsonRootPartition(disk)
}

func findJetsonRootPartition(disk string) (string, error) {
	_ = exec.Command("diskutil", "unmountDisk", disk).Run()
	output, errorValue := exec.Command("diskutil", "list", disk).Output()
	if errorValue != nil {
		return "", errorValue
	}
	diskIdentifier := filepath.Base(disk)
	var candidatePartitions []string
	fallbackPartition := ""
	fallbackSizeBytes := uint64(0)
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		identifier := fields[len(fields)-1]
		if !strings.HasPrefix(identifier, diskIdentifier+"s") {
			continue
		}
		partitionDevice := "/dev/" + identifier
		candidatePartitions = append(candidatePartitions, partitionDevice)
		if jetsonRootPartitionLooksValid(partitionDevice) {
			return partitionDevice, nil
		}
		if strings.Contains(line, "Linux Filesystem") {
			sizeBytes := diskutilSizeBytes(fields)
			if sizeBytes > fallbackSizeBytes {
				fallbackPartition = partitionDevice
				fallbackSizeBytes = sizeBytes
			}
		}
	}
	if fallbackPartition != "" && fallbackSizeBytes >= 4*1024*1024*1024 {
		fmt.Printf("  rootfs auto-detection fallback: %s\n", fallbackPartition)
		return fallbackPartition, nil
	}
	if len(candidatePartitions) > 0 {
		return "", fmt.Errorf("could not find Jetson Linux rootfs partition on %s; candidates: %s. Try --root-partition /dev/disk30s1", disk, strings.Join(candidatePartitions, ", "))
	}
	return "", fmt.Errorf("could not find Jetson Linux rootfs partition on %s", disk)
}

func diskutilSizeBytes(fields []string) uint64 {
	for index := 0; index < len(fields)-1; index++ {
		value, errorValue := strconv.ParseFloat(fields[index], 64)
		if errorValue != nil {
			continue
		}
		switch fields[index+1] {
		case "TB":
			return uint64(value * 1000 * 1000 * 1000 * 1000)
		case "GB":
			return uint64(value * 1000 * 1000 * 1000)
		case "MB":
			return uint64(value * 1000 * 1000)
		case "KB":
			return uint64(value * 1000)
		}
	}
	return 0
}

func jetsonRootPartitionLooksValid(partitionDevice string) bool {
	if _, errorValue := runDebugfs(partitionDevice, false, "stat /etc/passwd"); errorValue != nil {
		return false
	}
	if _, errorValue := runDebugfs(partitionDevice, false, "stat /etc/shadow"); errorValue != nil {
		return false
	}
	if _, errorValue := runDebugfs(partitionDevice, false, "stat /lib/systemd/system/multi-user.target"); errorValue != nil {
		return false
	}
	return true
}

func repairJetsonRootFilesystem(partitionDevice string) error {
	e2fsckBinary := "e2fsck"
	if binaryDirectory := e2fsprogsBinDir(); binaryDirectory != "" {
		e2fsckBinary = filepath.Join(binaryDirectory, "e2fsck")
	}
	fmt.Printf("  checking rootfs: %s\n", partitionDevice)
	command := exec.Command("sudo", "-n", e2fsckBinary, "-fy", partitionDevice)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if errorValue := command.Run(); errorValue != nil {
		if exitError, isExitError := errorValue.(*exec.ExitError); isExitError && exitError.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("repair Jetson rootfs %s failed: %w", partitionDevice, errorValue)
	}
	return nil
}

type jetsonRootPatch struct {
	partitionDevice string
	username        string
	password        string
	publicKey       string
	wifiProfiles    []resolvedWiFiProfile
}

func applyJetsonRootPatch(rootPatch jetsonRootPatch) error {
	rootFiles, errorValue := readJetsonAccountFiles(rootPatch.partitionDevice)
	if errorValue != nil {
		return errorValue
	}
	userID := nextLinuxUserID(rootFiles.passwd)
	passwordHash := "$6$internkim$39idW6Pq8VREb6WyArAK.m2UJK34FTq0TWjzbJNdnmKFM5ORh7gwCRrPLaCdcC19vs5uUwnURgBKX8TnqLnJh/"
	if rootPatch.password != jetsonDefaultPassword {
		return errors.New("--jetson-password currently supports the default value blueclaw only")
	}

	rootFiles.passwd = ensurePasswdUser(rootFiles.passwd, rootPatch.username, userID, userID)
	rootFiles.shadow = ensureShadowUser(rootFiles.shadow, rootPatch.username, passwordHash)
	rootFiles.group = ensureGroupUser(rootFiles.group, rootPatch.username, userID)
	rootFiles.gshadow = ensureGShadowUser(rootFiles.gshadow, rootPatch.username)

	if errorValue := writeJetsonAccountFiles(rootPatch.partitionDevice, rootFiles); errorValue != nil {
		return errorValue
	}
	if errorValue := createJetsonUserHome(rootPatch.partitionDevice, rootPatch.username, userID, rootPatch.publicKey); errorValue != nil {
		return errorValue
	}
	if errorValue := disableJetsonOEMConfig(rootPatch.partitionDevice); errorValue != nil {
		return errorValue
	}
	if errorValue := configureJetsonWiFiProfiles(rootPatch.partitionDevice, rootPatch.wifiProfiles); errorValue != nil {
		return errorValue
	}
	if errorValue := enableJetsonSSH(rootPatch.partitionDevice); errorValue != nil {
		return errorValue
	}
	if errorValue := installJetsonFirstboot(rootPatch.partitionDevice); errorValue != nil {
		return errorValue
	}
	return configureJetsonAutologin(rootPatch.partitionDevice, rootPatch.username)
}

type jetsonAccountFiles struct {
	passwd  string
	shadow  string
	group   string
	gshadow string
}

type jetsonPatchDocuments struct {
	hostname         string
	sshConfig        string
	oemMarker        string
	wifiConnection   string
	wifiSelector     string
	firstbootScript  string
	firstbootService string
	autologin        string
}

type jetsonDocumentExpectation struct {
	path         string
	document     string
	expectedText string
}

func readJetsonAccountFiles(partitionDevice string) (jetsonAccountFiles, error) {
	passwd, errorValue := dumpDebugfsFile(partitionDevice, "/etc/passwd")
	if errorValue != nil {
		return jetsonAccountFiles{}, errorValue
	}
	shadow, errorValue := dumpDebugfsFile(partitionDevice, "/etc/shadow")
	if errorValue != nil {
		return jetsonAccountFiles{}, errorValue
	}
	group, errorValue := dumpDebugfsFile(partitionDevice, "/etc/group")
	if errorValue != nil {
		return jetsonAccountFiles{}, errorValue
	}
	gshadow, _ := dumpDebugfsFile(partitionDevice, "/etc/gshadow")
	return jetsonAccountFiles{passwd: passwd, shadow: shadow, group: group, gshadow: gshadow}, nil
}

func writeJetsonAccountFiles(partitionDevice string, files jetsonAccountFiles) error {
	if errorValue := writeDebugfsContent(partitionDevice, "/etc/passwd", files.passwd, "0100644", 0, 0); errorValue != nil {
		return errorValue
	}
	if errorValue := writeDebugfsContent(partitionDevice, "/etc/shadow", files.shadow, "0100640", 0, 42); errorValue != nil {
		return errorValue
	}
	if errorValue := writeDebugfsContent(partitionDevice, "/etc/group", files.group, "0100644", 0, 0); errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(files.gshadow) != "" {
		return writeDebugfsContent(partitionDevice, "/etc/gshadow", files.gshadow, "0100640", 0, 42)
	}
	return nil
}

func createJetsonUserHome(partitionDevice string, username string, userID int, publicKey string) error {
	homePath := "/home/" + username
	_, _ = runDebugfs(partitionDevice, true, "mkdir /home")
	_, _ = runDebugfs(partitionDevice, true, "mkdir "+homePath)
	setDebugfsOwnership(partitionDevice, homePath, userID, userID, "040755")
	if errorValue := writeDebugfsContent(partitionDevice, homePath+"/.profile", "if [ -f ~/.bashrc ]; then\n  . ~/.bashrc\nfi\n", "0100644", userID, userID); errorValue != nil {
		return errorValue
	}
	if errorValue := writeDebugfsContent(partitionDevice, homePath+"/.bashrc", "export PATH=\"$HOME/.local/bin:$PATH\"\n", "0100644", userID, userID); errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(publicKey) == "" {
		return nil
	}
	sshDirectory := homePath + "/.ssh"
	_, _ = runDebugfs(partitionDevice, true, "mkdir "+sshDirectory)
	setDebugfsOwnership(partitionDevice, sshDirectory, userID, userID, "040700")
	return writeDebugfsContent(partitionDevice, sshDirectory+"/authorized_keys", strings.TrimSpace(publicKey)+"\n", "0100600", userID, userID)
}

func disableJetsonOEMConfig(partitionDevice string) error {
	removeDebugfsPaths(partitionDevice,
		"/etc/systemd/system/default.target",
		"/etc/systemd/system/display-manager.service",
		"/etc/systemd/system/nv-oem-config.service",
		"/etc/systemd/system/oem-config.service",
		"/etc/systemd/system/nv-oem-config.target",
		"/etc/systemd/system/oem-config.target",
		"/etc/systemd/system/graphical.target.wants/gdm.service",
		"/etc/systemd/system/graphical.target.wants/gdm3.service",
		"/etc/systemd/system/graphical.target.wants/lightdm.service",
		"/etc/systemd/system/graphical.target.wants/display-manager.service",
		"/etc/systemd/system/multi-user.target.wants/nv-oem-config.service",
		"/etc/systemd/system/multi-user.target.wants/oem-config.service",
		"/etc/systemd/system/graphical.target.wants/nv-oem-config.service",
		"/etc/systemd/system/graphical.target.wants/oem-config.service",
	)
	if _, errorValue := runDebugfs(partitionDevice, true, "symlink /etc/systemd/system/default.target /lib/systemd/system/multi-user.target"); errorValue != nil {
		return errorValue
	}
	_, _ = runDebugfs(partitionDevice, true, "symlink /etc/systemd/system/nv-oem-config.service /dev/null")
	_, _ = runDebugfs(partitionDevice, true, "symlink /etc/systemd/system/oem-config.service /dev/null")
	_, _ = runDebugfs(partitionDevice, true, "symlink /etc/systemd/system/display-manager.service /dev/null")
	if errorValue := writeDebugfsContent(partitionDevice, "/etc/nv-l4t-user-created", "1\n", "0100644", 0, 0); errorValue != nil {
		return errorValue
	}
	if errorValue := writeDebugfsContent(partitionDevice, "/etc/hostname", "internkim\n", "0100644", 0, 0); errorValue != nil {
		return errorValue
	}
	hostsDocument := "127.0.0.1 localhost\n127.0.1.1 internkim\n"
	if errorValue := writeDebugfsContent(partitionDevice, "/etc/hosts", hostsDocument, "0100644", 0, 0); errorValue != nil {
		return errorValue
	}
	return nil
}

func enableJetsonSSH(partitionDevice string) error {
	_, _ = runDebugfs(partitionDevice, true, "mkdir /etc/ssh/sshd_config.d")
	if errorValue := writeDebugfsContent(partitionDevice, "/etc/ssh/sshd_config.d/internkim.conf", "PasswordAuthentication yes\nPubkeyAuthentication yes\n", "0100644", 0, 0); errorValue != nil {
		return errorValue
	}
	_, _ = runDebugfs(partitionDevice, true, "mkdir /etc/systemd/system/multi-user.target.wants")
	_, _ = runDebugfs(partitionDevice, true, "symlink /etc/systemd/system/multi-user.target.wants/ssh.service /lib/systemd/system/ssh.service")
	return nil
}

func configureJetsonWiFiProfiles(partitionDevice string, profiles []resolvedWiFiProfile) error {
	if len(profiles) == 0 {
		return nil
	}
	removeDebugfsPaths(partitionDevice, "/etc/netplan/99-internkim-wifi.yaml", jetsonLegacyNetworkManagerConnectionPath)
	ensureDebugfsDirectories(partitionDevice, jetsonWiFiConnectionDirectory, filepath.Dir(jetsonWiFiSelectorScriptPath))
	for _, profile := range profiles {
		connectionDocument := buildJetsonNetworkManagerWiFiConnection(profile.SSID, profile.Password)
		if errorValue := writeDebugfsContent(partitionDevice, jetsonWiFiConnectionPath(profile.SSID), connectionDocument, "0100600", 0, 0); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := writeDebugfsContent(partitionDevice, jetsonWiFiSelectorScriptPath, buildJetsonWiFiSelectorScript()+"\n", "0100755", 0, 0); errorValue != nil {
		return errorValue
	}
	_, _ = runDebugfs(partitionDevice, true, "mkdir /etc/systemd/system/multi-user.target.wants")
	_, _ = runDebugfs(partitionDevice, true, "symlink /etc/systemd/system/multi-user.target.wants/NetworkManager.service /lib/systemd/system/NetworkManager.service")
	return nil
}

func buildJetsonNetworkManagerWiFiConnection(wifiSSID string, wifiPassword string) string {
	escapedSSID := escapeNetworkManagerValue(wifiSSID)
	escapedPassword := escapeNetworkManagerValue(wifiPassword)
	connectionID := jetsonWiFiConnectionID(wifiSSID)
	connectionUUID := jetsonWiFiConnectionUUID(wifiSSID)
	if wifiPassword == "" {
		return fmt.Sprintf(`[connection]
id=%s
uuid=%s
type=wifi
autoconnect=true

[wifi]
mode=infrastructure
hidden=true
ssid=%s

[ipv4]
method=auto

[ipv6]
method=auto
`, connectionID, connectionUUID, escapedSSID)
	}
	return fmt.Sprintf(`[connection]
id=%s
uuid=%s
type=wifi
autoconnect=true

[wifi]
mode=infrastructure
hidden=true
ssid=%s

[wifi-security]
key-mgmt=wpa-psk
psk=%s

[ipv4]
method=auto

[ipv6]
method=auto
`, connectionID, connectionUUID, escapedSSID, escapedPassword)
}

func escapeNetworkManagerValue(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, "\n", `\n`)
	return replacer.Replace(value)
}

func buildJetsonFirstbootScript() string {
	return strings.TrimSpace(`#!/bin/bash
set -euo pipefail
exec > /var/log/internkim-jetson-firstboot.log 2>&1

echo "=== Intern Kim Jetson firstboot ==="
date

mkdir -p /var/lib/internkim

expand_rootfs() {
  rootSource="$(findmnt -n -o SOURCE / 2>/dev/null || true)"
  if [ -z "$rootSource" ]; then
    echo "WARN: could not determine root filesystem source"
    return 0
  fi
  rootDisk="/dev/$(lsblk -no PKNAME "$rootSource" 2>/dev/null | head -1)"
  rootPartitionNumber="$(cat "/sys/class/block/$(basename "$rootSource")/partition" 2>/dev/null || true)"
  if [ ! -b "$rootDisk" ] || [ -z "$rootPartitionNumber" ]; then
    echo "WARN: could not determine root disk/partition for $rootSource"
    return 0
  fi

  availableMegabytes="$(df -Pm / | awk 'NR == 2 {print $4}')"
  if [ "${availableMegabytes:-0}" -ge 12000 ]; then
    echo "Root filesystem already has ${availableMegabytes}MB free"
    return 0
  fi

  echo "Expanding root filesystem on $rootSource via $rootDisk partition $rootPartitionNumber"
  if command -v growpart >/dev/null 2>&1; then
    growpart "$rootDisk" "$rootPartitionNumber" || true
  elif command -v parted >/dev/null 2>&1; then
    printf 'Yes\n' | parted ---pretend-input-tty "$rootDisk" resizepart "$rootPartitionNumber" 100% || true
  fi
  partprobe "$rootDisk" 2>/dev/null || true
  resize2fs "$rootSource" 2>/dev/null || true

  availableMegabytes="$(df -Pm / | awk 'NR == 2 {print $4}')"
  echo "Root filesystem free after resize: ${availableMegabytes:-unknown}MB"
}

unblock_wifi() {
  for rfkillPath in /sys/class/rfkill/rfkill*; do
    [ -d "$rfkillPath" ] || continue
    rfkillType="$(cat "$rfkillPath/type" 2>/dev/null || true)"
    if [ "$rfkillType" = "wlan" ]; then
      echo 0 > "$rfkillPath/soft" 2>/dev/null || true
    fi
  done
  command -v iw >/dev/null 2>&1 && iw reg set KR 2>/dev/null || true
}

wait_for_network_manager() {
  for attemptIndex in $(seq 1 30); do
    if command -v nmcli >/dev/null 2>&1 && nmcli general status >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  return 1
}

wait_for_ipv4() {
  for attemptIndex in $(seq 1 90); do
    boardIPAddress="$(ip -o -4 addr show scope global 2>/dev/null | awk '{split($4, parts, "/"); print parts[1]; exit}' || true)"
    if [ -n "$boardIPAddress" ]; then
      printf '%s' "$boardIPAddress" > /var/lib/internkim/board-ip
      mkdir -p /boot/internkim 2>/dev/null || true
      printf '%s' "$boardIPAddress" > /boot/internkim/board-ip 2>/dev/null || true
      echo "IPv4 ready: $boardIPAddress"
      return 0
    fi
    sleep 2
  done
  return 1
}

unblock_wifi
systemctl unmask NetworkManager.service 2>/dev/null || true
systemctl enable NetworkManager.service 2>/dev/null || true
systemctl start NetworkManager.service 2>/dev/null || true
systemctl enable ssh.service 2>/dev/null || systemctl enable sshd.service 2>/dev/null || true
systemctl restart ssh.service 2>/dev/null || systemctl restart sshd.service 2>/dev/null || true

if wait_for_network_manager; then
  nmcli connection reload 2>/dev/null || true
  nmcli radio wifi on 2>/dev/null || true
  if [ -x /usr/local/bin/internkim-wifi-select ]; then
    /usr/local/bin/internkim-wifi-select 2>/dev/null || true
  fi
else
  echo "WARN: NetworkManager did not become ready"
fi

if ! wait_for_ipv4; then
  echo "ERROR: no IPv4 address after firstboot wait"
  nmcli device status 2>/dev/null || true
  nmcli connection show 2>/dev/null || true
  ip addr 2>/dev/null || true
  exit 75
fi

expand_rootfs

touch /var/lib/internkim/jetson-firstboot.done
echo "Jetson firstboot complete"
`) + "\n"
}

func installJetsonFirstboot(partitionDevice string) error {
	if errorValue := writeDebugfsContent(partitionDevice, jetsonFirstbootScriptPath, buildJetsonFirstbootScript(), "0100755", 0, 0); errorValue != nil {
		return errorValue
	}
	serviceDocument := buildJetsonFirstbootService()
	if errorValue := writeDebugfsContent(partitionDevice, jetsonFirstbootServicePath, serviceDocument, "0100644", 0, 0); errorValue != nil {
		return errorValue
	}
	_, _ = runDebugfs(partitionDevice, true, "mkdir /etc/systemd/system/multi-user.target.wants")
	_, _ = runDebugfs(partitionDevice, true, "symlink /etc/systemd/system/multi-user.target.wants/internkim-jetson-firstboot.service "+jetsonFirstbootServicePath)
	return nil
}

func buildJetsonFirstbootService() string {
	return `[Unit]
Description=Intern Kim Jetson Firstboot
After=local-fs.target NetworkManager.service
Wants=NetworkManager.service
ConditionPathExists=!/var/lib/internkim/jetson-firstboot.done

[Service]
Type=oneshot
ExecStart=/usr/local/bin/internkim-jetson-firstboot.sh
Restart=on-failure
RestartSec=15
StartLimitIntervalSec=0
TimeoutStartSec=300
StandardOutput=journal+console
StandardError=journal+console

[Install]
WantedBy=multi-user.target
`
}

func configureJetsonAutologin(partitionDevice string, username string) error {
	ensureDebugfsDirectories(partitionDevice, "/etc/systemd/system/getty@tty1.service.d")
	return writeDebugfsContent(partitionDevice, jetsonAutologinOverridePath, buildJetsonAutologinOverride(username), "0100644", 0, 0)
}

func buildJetsonAutologinOverride(username string) string {
	return fmt.Sprintf(`[Service]
ExecStart=
ExecStart=-/sbin/agetty --autologin %s --noclear %%I $TERM
`, username)
}

func verifyJetsonRootPatch(rootPatch jetsonRootPatch) error {
	accountFiles, errorValue := readJetsonAccountFiles(rootPatch.partitionDevice)
	if errorValue != nil {
		return errorValue
	}
	documents := jetsonPatchDocuments{}
	if documents.hostname, errorValue = dumpDebugfsFile(rootPatch.partitionDevice, "/etc/hostname"); errorValue != nil {
		return fmt.Errorf("verify Jetson rootfs failed: read /etc/hostname: %w", errorValue)
	}
	if documents.sshConfig, errorValue = dumpDebugfsFile(rootPatch.partitionDevice, "/etc/ssh/sshd_config.d/internkim.conf"); errorValue != nil {
		return fmt.Errorf("verify Jetson rootfs failed: read /etc/ssh/sshd_config.d/internkim.conf: %w", errorValue)
	}
	if documents.oemMarker, errorValue = dumpDebugfsFile(rootPatch.partitionDevice, "/etc/nv-l4t-user-created"); errorValue != nil {
		return fmt.Errorf("verify Jetson rootfs failed: read /etc/nv-l4t-user-created: %w", errorValue)
	}
	if len(rootPatch.wifiProfiles) > 0 {
		wifiConnectionPath := jetsonWiFiConnectionPath(rootPatch.wifiProfiles[0].SSID)
		if documents.wifiConnection, errorValue = dumpDebugfsFile(rootPatch.partitionDevice, wifiConnectionPath); errorValue != nil {
			return fmt.Errorf("verify Jetson rootfs failed: read %s: %w", wifiConnectionPath, errorValue)
		}
		if documents.wifiSelector, errorValue = dumpDebugfsFile(rootPatch.partitionDevice, jetsonWiFiSelectorScriptPath); errorValue != nil {
			return fmt.Errorf("verify Jetson rootfs failed: read %s: %w", jetsonWiFiSelectorScriptPath, errorValue)
		}
	}
	if documents.firstbootScript, errorValue = dumpDebugfsFile(rootPatch.partitionDevice, jetsonFirstbootScriptPath); errorValue != nil {
		return fmt.Errorf("verify Jetson rootfs failed: read %s: %w", jetsonFirstbootScriptPath, errorValue)
	}
	if documents.firstbootService, errorValue = dumpDebugfsFile(rootPatch.partitionDevice, jetsonFirstbootServicePath); errorValue != nil {
		return fmt.Errorf("verify Jetson rootfs failed: read %s: %w", jetsonFirstbootServicePath, errorValue)
	}
	if documents.autologin, errorValue = dumpDebugfsFile(rootPatch.partitionDevice, jetsonAutologinOverridePath); errorValue != nil {
		return fmt.Errorf("verify Jetson rootfs failed: read %s: %w", jetsonAutologinOverridePath, errorValue)
	}
	return validateJetsonRootPatchDocuments(accountFiles, documents, rootPatch.username, rootPatch.wifiProfiles)
}

func validateJetsonRootPatchDocuments(accountFiles jetsonAccountFiles, documents jetsonPatchDocuments, username string, wifiProfiles []resolvedWiFiProfile) error {
	if !strings.Contains(accountFiles.passwd, username+":x:") {
		return fmt.Errorf("verify Jetson rootfs failed: missing user %s in /etc/passwd", username)
	}
	if !strings.Contains(accountFiles.shadow, username+":") {
		return fmt.Errorf("verify Jetson rootfs failed: missing user %s in /etc/shadow", username)
	}
	if !strings.Contains(accountFiles.group, "sudo:") || !strings.Contains(accountFiles.group, username) {
		return fmt.Errorf("verify Jetson rootfs failed: %s is not in sudo-capable groups", username)
	}
	for _, expectation := range []jetsonDocumentExpectation{
		{path: "/etc/hostname", document: documents.hostname, expectedText: "internkim"},
		{path: "/etc/ssh/sshd_config.d/internkim.conf", document: documents.sshConfig, expectedText: "PasswordAuthentication yes"},
		{path: "/etc/nv-l4t-user-created", document: documents.oemMarker, expectedText: "1"},
		{path: jetsonFirstbootScriptPath, document: documents.firstbootScript, expectedText: "Jetson firstboot complete"},
		{path: jetsonFirstbootServicePath, document: documents.firstbootService, expectedText: "internkim-jetson-firstboot.sh"},
		{path: jetsonAutologinOverridePath, document: documents.autologin, expectedText: "--autologin " + username},
	} {
		if !strings.Contains(expectation.document, expectation.expectedText) {
			return fmt.Errorf("verify Jetson rootfs failed: %s does not include %q", expectation.path, expectation.expectedText)
		}
	}
	if len(wifiProfiles) == 0 {
		return nil
	}
	wifiConnectionPath := jetsonWiFiConnectionPath(wifiProfiles[0].SSID)
	if !strings.Contains(documents.wifiConnection, "ssid="+escapeNetworkManagerValue(wifiProfiles[0].SSID)) {
		return fmt.Errorf("verify Jetson rootfs failed: %s does not include %q", wifiConnectionPath, "ssid="+escapeNetworkManagerValue(wifiProfiles[0].SSID))
	}
	if !strings.Contains(documents.wifiSelector, "records.sort") {
		return fmt.Errorf("verify Jetson rootfs failed: %s does not include Wi-Fi selection policy", jetsonWiFiSelectorScriptPath)
	}
	return nil
}

func ensureDebugfsDirectories(partitionDevice string, directoryPaths ...string) {
	for _, directoryPath := range directoryPaths {
		currentPath := ""
		for _, part := range strings.Split(strings.Trim(directoryPath, "/"), "/") {
			if part == "" {
				continue
			}
			currentPath += "/" + part
			_, _ = runDebugfs(partitionDevice, true, "mkdir "+currentPath)
		}
	}
}

func removeDebugfsPaths(partitionDevice string, paths ...string) {
	for _, path := range paths {
		_, _ = runDebugfs(partitionDevice, true, "rm "+path)
	}
}

func nextLinuxUserID(passwd string) int {
	nextUserID := 1000
	for _, line := range strings.Split(passwd, "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 3 {
			continue
		}
		userID, errorValue := strconv.Atoi(fields[2])
		if errorValue == nil && userID >= nextUserID {
			nextUserID = userID + 1
		}
	}
	return nextUserID
}

func ensurePasswdUser(passwd string, username string, userID int, groupID int) string {
	userLine := fmt.Sprintf("%s:x:%d:%d:Intern Kim:/home/%s:/bin/bash", username, userID, groupID, username)
	return replaceColonRecord(passwd, username, userLine)
}

func ensureShadowUser(shadow string, username string, passwordHash string) string {
	dayNumber := time.Now().Unix() / 86400
	userLine := fmt.Sprintf("%s:%s:%d:0:99999:7:::", username, passwordHash, dayNumber)
	return replaceColonRecord(shadow, username, userLine)
}

func ensureGroupUser(group string, username string, groupID int) string {
	group = replaceColonRecord(group, username, fmt.Sprintf("%s:x:%d:", username, groupID))
	for _, groupName := range []string{"adm", "sudo", "users", "video", "dialout", "plugdev", "input", "gpio", "i2c", "render"} {
		group = appendUserToColonRecordList(group, groupName, username)
	}
	return group
}

func ensureGShadowUser(gshadow string, username string) string {
	if strings.TrimSpace(gshadow) == "" {
		return gshadow
	}
	gshadow = replaceColonRecord(gshadow, username, fmt.Sprintf("%s:!::", username))
	for _, groupName := range []string{"adm", "sudo", "users", "video", "dialout", "plugdev", "input", "gpio", "i2c", "render"} {
		gshadow = appendUserToColonRecordList(gshadow, groupName, username)
	}
	return gshadow
}

func replaceColonRecord(document string, name string, replacement string) string {
	lines := splitTrimmedDocumentLines(document)
	replaced := false
	for index, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) > 0 && fields[0] == name {
			lines[index] = replacement
			replaced = true
		}
	}
	if !replaced {
		lines = append(lines, replacement)
	}
	return strings.Join(lines, "\n") + "\n"
}

func appendUserToColonRecordList(document string, recordName string, username string) string {
	lines := splitTrimmedDocumentLines(document)
	for index, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) < 4 || fields[0] != recordName {
			continue
		}
		members := splitCommaList(fields[3])
		for _, member := range members {
			if member == username {
				return strings.Join(lines, "\n") + "\n"
			}
		}
		members = append(members, username)
		fields[3] = strings.Join(members, ",")
		lines[index] = strings.Join(fields, ":")
		return strings.Join(lines, "\n") + "\n"
	}
	return strings.Join(lines, "\n") + "\n"
}

func splitTrimmedDocumentLines(document string) []string {
	var lines []string
	for _, line := range strings.Split(document, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func splitCommaList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	var values []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func dumpDebugfsFile(partitionDevice string, path string) (string, error) {
	temporaryFile, errorValue := os.CreateTemp("", "internkim-debugfs-dump-*")
	if errorValue != nil {
		return "", errorValue
	}
	temporaryPath := temporaryFile.Name()
	_ = temporaryFile.Close()
	defer os.Remove(temporaryPath)

	if _, errorValue := runDebugfs(partitionDevice, false, "dump "+path+" "+temporaryPath); errorValue != nil {
		return "", errorValue
	}
	document, errorValue := os.ReadFile(temporaryPath)
	return string(document), errorValue
}

func writeDebugfsContent(partitionDevice string, path string, content string, mode string, userID int, groupID int) error {
	temporaryFile, errorValue := os.CreateTemp("", "internkim-debugfs-write-*")
	if errorValue != nil {
		return errorValue
	}
	temporaryPath := temporaryFile.Name()
	if _, errorValue := temporaryFile.WriteString(content); errorValue != nil {
		_ = temporaryFile.Close()
		return errorValue
	}
	_ = temporaryFile.Close()
	defer os.Remove(temporaryPath)

	_, _ = runDebugfs(partitionDevice, true, "rm "+path)
	if _, errorValue := runDebugfs(partitionDevice, true, "write "+temporaryPath+" "+path); errorValue != nil {
		return errorValue
	}
	setDebugfsOwnership(partitionDevice, path, userID, groupID, mode)
	return nil
}

func setDebugfsOwnership(partitionDevice string, path string, userID int, groupID int, mode string) {
	_, _ = runDebugfs(partitionDevice, true, fmt.Sprintf("set_inode_field %s uid %d", path, userID))
	_, _ = runDebugfs(partitionDevice, true, fmt.Sprintf("set_inode_field %s gid %d", path, groupID))
	if mode != "" {
		_, _ = runDebugfs(partitionDevice, true, fmt.Sprintf("set_inode_field %s mode %s", path, mode))
	}
}

func runDebugfs(partitionDevice string, isWritable bool, command string) (string, error) {
	debugfsBinary := "debugfs"
	if binaryDirectory := e2fsprogsBinDir(); binaryDirectory != "" {
		debugfsBinary = filepath.Join(binaryDirectory, "debugfs")
	}
	arguments := []string{debugfsBinary}
	if isWritable {
		arguments = append(arguments, "-w")
	}
	arguments = append(arguments, "-R", command, partitionDevice)
	sudoArguments := append([]string{"-n"}, arguments...)
	output, errorValue := exec.Command("sudo", sudoArguments...).CombinedOutput()
	outputText := string(output)
	if errorValue != nil {
		return outputText, fmt.Errorf("debugfs %s: %s: %w", command, strings.TrimSpace(outputText), errorValue)
	}
	for _, failureText := range []string{"No such file or directory", "File not found by ext2_lookup", "Filesystem not open"} {
		if strings.Contains(outputText, failureText) {
			return outputText, fmt.Errorf("debugfs %s: %s", command, strings.TrimSpace(outputText))
		}
	}
	return outputText, nil
}
