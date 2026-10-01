package cli

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	setup "github.com/yeomyeonggeori/internkim/internal/provisioning/steps"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// backupFromExt4 extracts workspace files from the SD card's ext4 partition
// using debugfs (file-by-file, no full partition copy).
func backupFromExt4(disk, backupDir string, messenger *msg) {
	partDevice := ""
	out, _ := exec.Command("diskutil", "list", disk).Output()
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "Linux") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				partDevice = "/dev/" + fields[len(fields)-1]
			}
		}
	}
	if partDevice == "" {
		return
	}
	debugfsBin := filepath.Join(e2fsprogsBinDir(), "debugfs")
	if e2fsprogsBinDir() == "" {
		debugfsBin = "debugfs"
	}
	fmt.Printf("    %s\n", messenger.t("워크스페이스 백업 중 (debugfs)...", "Backing up workspace (debugfs)..."))
	// List workspace files
	listCommand := exec.Command("sudo", debugfsBin, "-R", "ls -p /root/.blueclaw/workspace", partDevice)
	listOutput, err := listCommand.Output()
	if err != nil {
		fmt.Printf("    %s\n", messenger.t("워크스페이스 읽기 실패 — 건너뜀", "Failed to read workspace — skipping"))
		return
	}
	wsBackupDir := filepath.Join(backupDir, "workspace")
	os.RemoveAll(wsBackupDir)
	os.MkdirAll(wsBackupDir, 0755)
	for _, line := range strings.Split(string(listOutput), "\n") {
		parts := strings.Split(line, "/")
		if len(parts) < 7 {
			continue
		}
		name := parts[5]
		if name == "" || name == "." || name == ".." || name == "bin" || name == "downloads" {
			continue
		}
		statCommand := exec.Command("sudo", debugfsBin, "-R",
			fmt.Sprintf("stat /root/.blueclaw/workspace/%s", name), partDevice)
		statOutput, err := statCommand.Output()
		if err != nil || !strings.Contains(string(statOutput), "Type: regular") {
			continue
		}
		destPath := filepath.Join(wsBackupDir, name)
		dumpCommand := exec.Command("sudo", debugfsBin, "-R",
			fmt.Sprintf("dump /root/.blueclaw/workspace/%s %s", name, destPath), partDevice)
		dumpCommand.Run()
	}
	// Count backed up files
	entries, _ := os.ReadDir(wsBackupDir)
	if len(entries) > 0 {
		fmt.Printf("    %s (%d files)\n", messenger.t("워크스페이스 백업 완료", "Workspace backed up"), len(entries))
	}
}

var (
	// Armbian Trixie Minimal images per board
	armbianImages = map[string]string{
		"rpi":       "https://dl.armbian.com/rpi4b/Trixie_current_minimal",     // RPi 3/4/5
		"orangepi5": "https://dl.armbian.com/orangepi5/Trixie_current_minimal", // Orange Pi 5 (RK3588S)
	}
)

func runSetupSD(m *msg) {
	cfg := loadConfig()
	stateDir := internkimHomeDir()
	scriptDir, _ := os.Getwd()
	hardReset := containsArg("--hard-reset")
	reset := hardReset || containsArg("--reset")
	fromStep := argInt("--from", 0)
	shouldRun := func(n int) bool { return fromStep == 0 || n >= fromStep }
	totalSteps := 8

	// Board selection
	boardType := "rpi" // default
	if containsArg("--board") {
		for i, a := range os.Args {
			if a == "--board" && i+1 < len(os.Args) {
				boardType = os.Args[i+1]
			}
		}
	}
	imageURL, ok := armbianImages[boardType]
	if !ok {
		fmt.Printf("지원 보드: ")
		for k := range armbianImages {
			fmt.Printf("%s ", k)
		}
		fmt.Println()
		fatal(fmt.Sprintf("알 수 없는 보드: %s", boardType))
	}
	boardNames := map[string]string{"rpi": "CM5", "orangepi5": "Orange Pi 5"}
	fmt.Printf("=== Intern Kim Setup (%s, Armbian Trixie) ===\n", boardNames[boardType])
	fmt.Println()

	// Stop the lab VM if it is running so the same Cloudflare tunnel
	// token cannot race between the VM and the real device.
	if labVirtualMachineIPAddress := resolveLabVirtualMachineIPAddress(); labVirtualMachineIPAddress != "" {
		fmt.Printf("  %s\n", m.t("Lab VM 중지 중 (터널 충돌 방지)...", "Stopping lab VM (tunnel conflict)..."))
		if errorValue := runLabArguments([]string{"vm-down"}); errorValue != nil {
			fmt.Printf("  %s: %v\n", m.t("Lab VM 중지 실패", "Failed to stop lab VM"), errorValue)
		} else {
			fmt.Printf("  %s\n", m.t("Lab VM 중지 완료", "Lab VM stopped"))
		}
	}

	// 1. SD card detection (always needed)
	step(1, totalSteps, m.t("SD 카드 감지 중...", "Detecting SD card..."))
	disk := detectSDCard()
	if disk == "" {
		fatal(m.t(
			"SD 카드를 찾을 수 없습니다.\nSD 카드를 삽입한 후 다시 시도하세요.",
			"SD card not found.\nInsert an SD card and try again.",
		))
	}
	fmt.Printf("  %s: %s\n", m.t("SD 카드 감지", "SD card detected"), disk)

	cacheDir := filepath.Join(stateDir, "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		fatal(err.Error())
	}

	setupBuildID := currentExecutableFingerprint()
	stageDir := filepath.Join(cacheDir, "stage")
	stageBuildIDPath := filepath.Join(stageDir, "setup-build-id")
	if reset {
		_ = os.RemoveAll(stageDir)
	} else if setupBuildID != "" {
		stageBuildIDBytes, err := os.ReadFile(stageBuildIDPath)
		if err != nil || strings.TrimSpace(string(stageBuildIDBytes)) != setupBuildID {
			_ = os.RemoveAll(stageDir)
		}
	}
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		fatal(err.Error())
	}

	flowState := newSetupFlowState(
		m,
		cfg,
		collectSetupParameterValues(),
		stateDir,
		scriptDir,
		setupBuildID,
		nil,
		containsArg("--non-interactive"),
	)
	stageContext := &setup.Context{
		Backend:   setup.BackendSD,
		Language:  m.lang,
		StateDir:  stateDir,
		ScriptDir: scriptDir,
		Force:     reset || containsArg("--force"),
		Callbacks: flowState.callbacks(),
		SD:        sdStagingTarget{stagingRoot: stageDir},
	}
	stageRegistry := setup.DefaultRegistry()
	runStageStep := func(stepNumber int, stepName string) {
		selector := setup.Selector{
			Only:  []string{stepName},
			Force: stageContext.Force && shouldRun(stepNumber),
		}
		if err := stageRegistry.Run(stageContext, selector); err != nil {
			fatal(err.Error())
		}
	}

	step(2, totalSteps, m.t("Wi-Fi 설정...", "Wi-Fi setup..."))
	runStageStep(2, "wifi")

	step(3, totalSteps, m.t("OpenRouter API 키 설정...", "OpenRouter API key..."))
	runStageStep(3, "openrouter")

	step(4, totalSteps, m.t("기기 등록 + 터널 설정...", "Registering device + tunnel..."))
	runStageStep(4, "tunnel")

	step(5, totalSteps, m.t("부팅 스테이지 준비...", "Preparing boot staging payload..."))
	runStageStep(5, "staging")

	if err := flowState.ensureWiFiCredentials(); err != nil {
		fatal(err.Error())
	}

	// The image is not reflashed when the SD card already carries it.
	step(6, totalSteps, m.t("이미지 굽기...", "Flashing image..."))

	imgBase := fmt.Sprintf("armbian-%s-trixie", boardType)
	imgXZ := filepath.Join(cacheDir, imgBase+".img.xz")
	imgRaw := filepath.Join(cacheDir, imgBase+".img")
	// Download compressed image early (needed for rootfs-based deb download)
	if _, err := os.Stat(imgXZ); os.IsNotExist(err) {
		fmt.Printf("  %s...\n", m.t("Armbian trixie 이미지 다운로드 중", "Downloading Armbian trixie image"))
		downloadCommand := exec.Command("curl", "-fSL", "--progress-bar", "-o", imgXZ, "-L", imageURL)
		downloadCommand.Stdout = os.Stdout
		downloadCommand.Stderr = os.Stderr
		if err := downloadCommand.Run(); err != nil {
			os.Remove(imgXZ)
			fatal(m.t("이미지 다운로드 실패.", "Image download failed."))
		}
	}
	// Pre-download .deb packages using Armbian rootfs chroot (version-matched)
	debsTarPath := filepath.Join(cacheDir, "debs.tar")
	if _, err := os.Stat(debsTarPath); os.IsNotExist(err) {
		fmt.Printf("  %s\n", m.t("패키지 사전 다운로드 (Armbian rootfs)", "Pre-downloading packages (Armbian rootfs)"))
		if err := downloadDebsUsingRootfs(imgXZ, debsTarPath, m); err != nil {
			fmt.Printf("  FAILED: %v\n", err)
		} else {
			info, _ := os.Stat(debsTarPath)
			fmt.Printf("  %s (%dMB)\n", m.t("패키지 다운로드 완료", "Package download complete"), info.Size()/1024/1024)
		}
	} else {
		info, _ := os.Stat(debsTarPath)
		fmt.Printf("  %s (%dMB)\n", m.t("패키지 캐시 사용", "Using cached packages"), info.Size()/1024/1024)
	}

	// Check if SD already has the same image and the same setup build.
	exec.Command("diskutil", "mountDisk", disk).Run()
	time.Sleep(2 * time.Second)
	sdSameImage := false
	sdSameSetupBuild := false
	for _, d := range []string{"/Volumes/NO NAME", "/Volumes/RASPIFIRM", "/Volumes/RPICFG", "/Volumes/boot", "/Volumes/bootfs", "/Volumes/armbi_root"} {
		stageDir := filepath.Join(d, "internkim")
		imageMarker := filepath.Join(stageDir, "image-version")
		if data, err := os.ReadFile(imageMarker); err == nil && string(data) == imageURL {
			sdSameImage = true
			if setupBuildID == "" {
				sdSameSetupBuild = true
				break
			}
			buildMarker := filepath.Join(stageDir, "setup-build-id")
			if data, err := os.ReadFile(buildMarker); err == nil && strings.TrimSpace(string(data)) == setupBuildID {
				sdSameSetupBuild = true
				break
			}
		}
	}

	needFlash := reset || !sdSameImage || !sdSameSetupBuild || flowState.wifiChanged
	if !needFlash {
		fmt.Printf("  %s\n", m.t("동일 이미지 감지 — 굽기 건너뜀 (boot 파티션만 업데이트)", "Same image — skipping flash (boot partition update only)"))
	} else {
		if !reset && sdSameImage && !sdSameSetupBuild && !flowState.wifiChanged {
			fmt.Printf("  %s\n", m.t("새 setup 빌드 감지 — 다시 굽기", "New setup build detected — reflashing"))
		}
		fmt.Println()
		if !promptYN(m.t(
			disk+" 의 모든 데이터가 삭제됩니다. 계속하시겠습니까?",
			"All data on "+disk+" will be erased. Continue?",
		)) {
			fatal(m.t("취소됨.", "Cancelled."))
		}
		// Backup data before flash (unless --hard-reset)
		backupDir := filepath.Join(stateDir, "backup")
		if !hardReset {
			os.MkdirAll(backupDir, 0700)
			// Workspace + DB from ext4 via debugfs (file-by-file, no full dd)
			backupFromExt4(disk, backupDir, m)
			fmt.Printf("  %s\n", m.t("백업 완료", "Backup complete"))
		} else {
			os.RemoveAll(backupDir)
		}

		// Always extract fresh image (inject needs clean ext4)
		os.Remove(imgRaw)
		fmt.Printf("  %s...\n", m.t("이미지 압축 해제 중 (img.xz → img)", "Extracting image (img.xz → img)"))
		xzCmd := exec.Command("sh", "-c", fmt.Sprintf("xz -dk '%s'", imgXZ))
		xzCmd.Stderr = os.Stderr
		if err := xzCmd.Run(); err != nil {
			os.Remove(imgRaw)
			fatal(m.t("압축 해제 실패.", "Extraction failed."))
		}

		// Inject Wi-Fi, SSH, hostname, firstboot service into ext4 via debugfs
		fmt.Printf("  %s...\n", m.t("이미지에 파일 주입 중 (debugfs)", "Injecting files into image (debugfs)"))
		consolePassword, err := resolveConsolePassword()
		if err != nil {
			fatal(err.Error())
		}
		if err := injectFilesIntoImage(imgRaw, flowState.wifiSSID, flowState.wifiPassword, flowState.publicKey, consolePassword, stageDir); err != nil {
			fatal(fmt.Sprintf("%s: %v", m.t("파일 주입 실패", "File injection failed"), err))
		}
		if err := saveConsolePassword(consolePassword); err != nil {
			fatal(err.Error())
		}
		fmt.Printf("  %s: %s\n", m.t("root·internkim 콘솔 비밀번호", "root and internkim console password"), consolePassword)
		fmt.Printf("  %s\n", m.t("파일 주입 완료", "Files injected"))

		// Write to SD
		fmt.Printf("  %s...\n", m.t("SD 카드에 이미지 쓰는 중 (수 분 소요)", "Writing image to SD (may take a few minutes)"))
		exec.Command("diskutil", "unmountDisk", disk).Run()
		rdisk := strings.Replace(disk, "/dev/disk", "/dev/rdisk", 1)
		ddCmd := exec.Command("sudo", "dd", "if="+imgRaw, "of="+rdisk, "bs=4m", "status=progress")
		ddCmd.Stdout = os.Stdout
		ddCmd.Stderr = os.Stderr
		if err := ddCmd.Run(); err != nil {
			fatal(m.t("이미지 쓰기 실패.", "Failed to write image."))
		}
		exec.Command("sync").Run()
	}

	// macOS cannot mount the ext4 root partition, so everything goes on the
	// FAT32 boot partition. The first-boot script moves files into place.
	step(7, totalSteps, m.t("파일 주입 중...", "Injecting files..."))
	exec.Command("diskutil", "mountDisk", disk).Run()
	time.Sleep(2 * time.Second)

	bootDir := ""
	for _, d := range []string{"/Volumes/NO NAME", "/Volumes/RASPIFIRM", "/Volumes/RPICFG", "/Volumes/boot", "/Volumes/bootfs"} {
		if _, err := os.Stat(d); err == nil {
			bootDir = d
			break
		}
	}
	if bootDir == "" {
		fatal(m.t("boot 파티션을 마운트할 수 없습니다.", "Could not mount boot partition."))
	}

	bootStageDir := filepath.Join(bootDir, "internkim")
	_ = os.RemoveAll(bootStageDir)
	if err := os.MkdirAll(bootStageDir, 0o755); err != nil {
		fatal(err.Error())
	}
	if err := copyDirectoryContents(stageDir, bootStageDir); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("  %s\n", m.t("스테이지 디렉터리 복사 완료", "Stage directory copied"))

	sysconf := "hostname=internkim\n"
	if flowState.publicKey != "" {
		sysconf += fmt.Sprintf("root_authorized_key=%s\n", flowState.publicKey)
	}
	os.WriteFile(filepath.Join(bootDir, "sysconf.txt"), []byte(sysconf), 0644)
	fmt.Printf("  %s\n", m.t("sysconf.txt 작성 완료", "sysconf.txt written"))

	// Image version marker (skip re-flash next time if same image)
	os.WriteFile(filepath.Join(bootStageDir, "image-version"), []byte(imageURL), 0644)
	if setupBuildID != "" {
		os.WriteFile(filepath.Join(bootStageDir, "setup-build-id"), []byte(setupBuildID), 0644)
	}
	// Build ID (unique per flash, shown in firstboot log)
	buildID := fmt.Sprintf("%x", time.Now().UnixNano())
	os.WriteFile(filepath.Join(bootStageDir, "build-id"), []byte(buildID), 0644)
	fmt.Printf("  Build ID: %s\n", buildID)

	// No cmdline.txt modification needed — systemd service is injected into root partition via container

	// Save local subnet for board discovery
	if out, err := exec.Command("sh", "-c", "route get default 2>/dev/null | awk '/gateway/{print $2}'").Output(); err == nil {
		gw := strings.TrimSpace(string(out))
		if parts := strings.Split(gw, "."); len(parts) == 4 {
			subnet := strings.Join(parts[:3], ".")
			saveState(stateDir, "subnet", subnet)
		}
	}

	step(8, totalSteps, m.t("완료!", "Done!"))
	exec.Command("diskutil", "unmountDisk", disk).Run()

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("  %s\n", m.t("SD 카드 준비 완료!", "SD card ready!"))
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println(m.t(
		"  1. SD 카드를 보드에 삽입\n  2. 전원 연결\n  3. 첫 부팅 시 자동 설정 (약 5-10분 소요)",
		"  1. Insert the SD card into the board\n  2. Connect power\n  3. First boot auto-setup (takes ~5-10 min)",
	))
	deviceURL := loadState(stateDir, "device_url")
	if deviceURL != "" {
		fmt.Printf("\n  %s: %s\n", m.t("주소", "Address"), deviceURL)
	}
	fmt.Println()
}

// detectSDCard finds an external physical disk (not disk images) on macOS.
func detectSDCard() string {
	out, err := exec.Command("diskutil", "list", "external").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		// Look for lines like: /dev/disk22 (external, physical):
		if strings.Contains(line, "(external, physical)") {
			parts := strings.Fields(line)
			if len(parts) > 0 && strings.HasPrefix(parts[0], "/dev/disk") {
				return parts[0]
			}
		}
	}
	return ""
}

func localScanSubnets(stateDirectory string) []string {
	subnetSet := make(map[string]bool)
	if storedSubnet := strings.TrimSpace(loadState(stateDirectory, "subnet")); storedSubnet != "" {
		subnetSet[storedSubnet] = true
	}
	interfaces, errorValue := net.Interfaces()
	if errorValue == nil {
		for _, networkInterface := range interfaces {
			if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addresses, addressError := networkInterface.Addrs()
			if addressError != nil {
				continue
			}
			for _, address := range addresses {
				ipNet, ok := address.(*net.IPNet)
				if !ok {
					continue
				}
				ipv4 := ipNet.IP.To4()
				if ipv4 == nil || ipv4.IsLoopback() || ipv4.IsLinkLocalUnicast() {
					continue
				}
				subnetSet[fmt.Sprintf("%d.%d.%d", ipv4[0], ipv4[1], ipv4[2])] = true
			}
		}
	}
	if output, errorValue := exec.Command("sh", "-c", "route get default 2>/dev/null | awk '/gateway/{print $2}'").Output(); errorValue == nil {
		gateway := strings.TrimSpace(string(output))
		if parts := strings.Split(gateway, "."); len(parts) == 4 {
			subnetSet[strings.Join(parts[:3], ".")] = true
		}
	}
	subnets := make([]string, 0, len(subnetSet))
	for subnet := range subnetSet {
		subnets = append(subnets, subnet)
	}
	sort.Strings(subnets)
	return subnets
}

func findExt4Partition(imgRaw string) (offset, size int64, err error) {
	f, err := os.Open(imgRaw)
	if err != nil {
		return 0, 0, fmt.Errorf("open image: %w", err)
	}
	defer f.Close()
	buf := make([]byte, 16)
	for i := 0; i < 4; i++ {
		f.Seek(446+int64(i*16), 0)
		f.Read(buf)
		partitionType := buf[4]
		lba := int64(buf[8]) | int64(buf[9])<<8 | int64(buf[10])<<16 | int64(buf[11])<<24
		sectors := int64(buf[12]) | int64(buf[13])<<8 | int64(buf[14])<<16 | int64(buf[15])<<24
		if partitionType == 0x83 || (partitionType == 0xee && lba > 2048 && sectors > 100000) {
			return lba * 512, sectors * 512, nil
		}
	}
	return 0, 0, fmt.Errorf("could not find ext4 partition in image")
}

func e2fsprogsBinDir() string {
	homebrewPath := "/opt/homebrew/Cellar/e2fsprogs/1.47.4/sbin"
	if _, err := os.Stat(filepath.Join(homebrewPath, "debugfs")); err == nil {
		return homebrewPath
	}
	return ""
}

func downloadDebsUsingRootfs(imgXZ, debsTarPath string, messenger *msg) error {
	tempImg, err := os.CreateTemp("", "armbian-*.img")
	if err != nil {
		return fmt.Errorf("create temp image: %w", err)
	}
	tempImgPath := tempImg.Name()
	tempImg.Close()
	defer os.Remove(tempImgPath)
	fmt.Printf("    %s\n", messenger.t("이미지 압축 해제 중...", "Extracting image..."))
	extractCommand := exec.Command("sh", "-c", fmt.Sprintf("xz -dc '%s' > '%s'", imgXZ, tempImgPath))
	extractCommand.Stderr = os.Stderr
	if err := extractCommand.Run(); err != nil {
		return fmt.Errorf("extract xz: %w", err)
	}
	partitionOffset, partitionSize, err := findExt4Partition(tempImgPath)
	if err != nil {
		return err
	}
	rootfsDirectory, err := os.MkdirTemp("", "armbian-rootfs-*")
	if err != nil {
		return fmt.Errorf("create rootfs dir: %w", err)
	}
	defer os.RemoveAll(rootfsDirectory)
	rootfsPath := filepath.Join(rootfsDirectory, "rootfs.ext4")
	fmt.Printf("    %s\n", messenger.t("rootfs 파티션 추출 중...", "Extracting rootfs partition..."))
	ddCommand := exec.Command("dd",
		fmt.Sprintf("if=%s", tempImgPath),
		fmt.Sprintf("of=%s", rootfsPath),
		"bs=4096",
		fmt.Sprintf("skip=%d", partitionOffset/4096),
		fmt.Sprintf("count=%d", partitionSize/4096),
	)
	ddCommand.Stderr = os.Stderr
	if err := ddCommand.Run(); err != nil {
		return fmt.Errorf("extract rootfs: %w", err)
	}
	os.Remove(tempImgPath)
	expandBytes := int64(512 * 1024 * 1024)
	if f, err := os.OpenFile(rootfsPath, os.O_WRONLY, 0); err == nil {
		f.Truncate(partitionSize + expandBytes)
		f.Close()
	}
	binDir := e2fsprogsBinDir()
	resize2fsBin := "resize2fs"
	if binDir != "" {
		resize2fsBin = filepath.Join(binDir, "resize2fs")
	}
	exec.Command(resize2fsBin, "-f", rootfsPath).CombinedOutput()
	fmt.Printf("    %s\n", messenger.t("Armbian rootfs에서 패키지 다운로드 중...", "Downloading packages from Armbian rootfs..."))
	chrootScript := fmt.Sprintf(`set -e
apt-get update -qq >/dev/null 2>&1
apt-get install -y -qq e2fsprogs >/dev/null 2>&1
mkdir -p /mnt/armbian
mount -o loop,rw /mnt/host/rootfs.ext4 /mnt/armbian
mount -t proc proc /mnt/armbian/proc
mount --bind /dev /mnt/armbian/dev
rm -f /mnt/armbian/etc/resolv.conf
echo "nameserver 8.8.8.8" > /mnt/armbian/etc/resolv.conf
chroot /mnt/armbian sh -c 'export DEBIAN_FRONTEND=noninteractive; apt-get update -qq >/dev/null 2>&1; . /etc/os-release; runtimePackages="%s"; case "${VERSION_ID:-}" in 22.*) runtimePackages="%s" ;; 24.*|25.*|26.*) runtimePackages="%s" ;; esac; apt-get install -y -d -qq %s jq avahi-daemon git curl unzip ca-certificates $runtimePackages >/dev/null 2>&1'
tar cf - -C /mnt/armbian/var/cache/apt/archives .
umount /mnt/armbian/dev /mnt/armbian/proc 2>/dev/null; umount /mnt/armbian 2>/dev/null; true`, deviceBrowserRuntimePackageListLegacyUbuntu(), deviceBrowserRuntimePackageListJetPack6(), deviceBrowserRuntimePackageListUbuntu24(), blueclaw.BuzzRelayDatabasePackages())
	debCommand := exec.Command("container", "run", "--rm",
		"--volume", rootfsDirectory+":/mnt/host",
		"debian:trixie-slim", "sh", "-c", chrootScript)
	debOutput, err := os.Create(debsTarPath)
	if err != nil {
		return fmt.Errorf("create debs.tar: %w", err)
	}
	debCommand.Stdout = debOutput
	if err := debCommand.Run(); err != nil {
		debOutput.Close()
		os.Remove(debsTarPath)
		return fmt.Errorf("container deb download: %w", err)
	}
	debOutput.Close()
	return nil
}

func injectFilesIntoImage(imgRaw, ssid, wifiPass, pubKey, consolePassword, stageDir string) error {
	binDir := e2fsprogsBinDir()
	debugfsBin := "debugfs"
	if binDir != "" {
		debugfsBin = filepath.Join(binDir, "debugfs")
	}
	partOffset, partSize, err := findExt4Partition(imgRaw)
	if err != nil {
		return err
	}

	// Extract ext4 partition to temp file
	partFile := imgRaw + ".rootfs"
	extractCmd := exec.Command("dd",
		fmt.Sprintf("if=%s", imgRaw),
		fmt.Sprintf("of=%s", partFile),
		"bs=4096",
		fmt.Sprintf("skip=%d", partOffset/4096),
		fmt.Sprintf("count=%d", partSize/4096),
	)
	extractCmd.Stderr = os.Stderr
	if err := extractCmd.Run(); err != nil {
		return fmt.Errorf("extract partition: %w", err)
	}
	defer os.Remove(partFile)

	// Expand ext4 partition to fit injected files
	expandMB := int64(1024) // 1GB extra space
	f2, _ := os.OpenFile(partFile, os.O_WRONLY, 0)
	if f2 != nil {
		f2.Seek(0, 2) // end
		f2.Truncate(partSize + expandMB*1024*1024)
		f2.Close()
	}
	resize2fsBin := filepath.Join(filepath.Dir(debugfsBin), "resize2fs")
	if out, err := exec.Command(resize2fsBin, "-f", partFile).CombinedOutput(); err != nil {
		fmt.Printf("    resize2fs warning: %s\n", string(out))
	}
	partSize = partSize + expandMB*1024*1024

	dbgRun := func(cmd string) {
		exec.Command(debugfsBin, "-w", "-R", cmd, partFile).CombinedOutput()
	}

	ensureDirs := func(ext4Path string) {
		parts := strings.Split(filepath.Dir(ext4Path), "/")
		cur := ""
		for _, p := range parts {
			if p == "" {
				continue
			}
			cur += "/" + p
			dbgRun(fmt.Sprintf("mkdir %s", cur))
		}
	}

	writeFile := func(localPath, ext4Path string, mode string) error {
		ensureDirs(ext4Path)
		dbgRun(fmt.Sprintf("rm %s", ext4Path))
		// debugfs returns exit 0 even on failure, so check output for errors
		cmd := exec.Command(debugfsBin, "-w", "-R", fmt.Sprintf("write %s %s", localPath, ext4Path), partFile)
		out, err := cmd.CombinedOutput()
		outStr := string(out)
		if err != nil {
			return fmt.Errorf("debugfs write %s: %s", ext4Path, outStr)
		}
		if strings.Contains(outStr, "already exists") || strings.Contains(outStr, "No space") {
			return fmt.Errorf("debugfs write %s: %s", ext4Path, outStr)
		}
		if mode != "" {
			dbgRun(fmt.Sprintf("set_inode_field %s mode %s", ext4Path, mode))
		}
		return nil
	}

	writeContent := func(content, ext4Path, mode string) error {
		tmp, _ := os.CreateTemp("", "inject-*")
		tmp.WriteString(content)
		tmp.Sync()
		tmp.Close()
		defer os.Remove(tmp.Name())
		return writeFile(tmp.Name(), ext4Path, mode)
	}

	mkSymlink := func(linkPath, target string) {
		ensureDirs(linkPath)
		dbgRun(fmt.Sprintf("symlink %s %s", linkPath, target))
	}

	// ── 1. Wi-Fi (wpa_supplicant@wlan0 + systemd-networkd) ──
	if ssid != "" {
		fmt.Println("    Wi-Fi config")
		var wpaConf string
		if wifiPass != "" {
			wpaConf = fmt.Sprintf("ctrl_interface=DIR=/run/wpa_supplicant GROUP=netdev\ncountry=KR\nap_scan=1\nnetwork={\n  ssid=\"%s\"\n  scan_ssid=1\n  key_mgmt=WPA-PSK\n  psk=\"%s\"\n}\n", ssid, wifiPass)
		} else {
			wpaConf = fmt.Sprintf("ctrl_interface=DIR=/run/wpa_supplicant GROUP=netdev\ncountry=KR\nap_scan=1\nnetwork={\n  ssid=\"%s\"\n  scan_ssid=1\n  key_mgmt=NONE\n}\n", ssid)
		}
		writeContent(wpaConf, "/etc/wpa_supplicant/wpa_supplicant-wlan0.conf", "0100600")
		// systemd-networkd DHCP for wlan0
		writeContent("[Match]\nName=wlan0\n\n[Network]\nDHCP=yes\n\n[DHCPv4]\nRouteMetric=20\n", "/etc/systemd/network/20-wlan0.network", "0100644")
		// Enable wpa_supplicant@wlan0 (global service will be masked at runtime by firstboot)
		mkSymlink("/etc/systemd/system/multi-user.target.wants/wpa_supplicant@wlan0.service",
			"/usr/lib/systemd/system/wpa_supplicant@.service")
	}

	// ── 2. Armbian first-run auto-config (skip interactive root password prompt) ──
	fmt.Println("    armbian first-run config")
	armbianConf := "PRESET_NET_CHANGE_DEFAULTS=\"1\"\n"
	armbianConf += "PRESET_NET_WIFI_ENABLED=\"1\"\n"
	armbianConf += fmt.Sprintf("PRESET_NET_WIFI_SSID=\"%s\"\n", ssid)
	armbianConf += fmt.Sprintf("PRESET_NET_WIFI_KEY=\"%s\"\n", wifiPass)
	armbianConf += "PRESET_NET_WIFI_COUNTRYCODE=\"KR\"\n"
	armbianConf += "PRESET_CONNECT_WIRELESS=\"n\"\n"
	armbianConf += "SET_LANG_BASED_ON_LOCATION=\"n\"\n"
	armbianConf += "PRESET_LOCALE=\"en_US.UTF-8\"\n"
	armbianConf += "PRESET_TIMEZONE=\"Asia/Seoul\"\n"
	armbianConf += fmt.Sprintf("PRESET_ROOT_PASSWORD=\"%s\"\n", consolePassword)
	armbianConf += "PRESET_USER_NAME=\"internkim\"\n"
	armbianConf += fmt.Sprintf("PRESET_USER_PASSWORD=\"%s\"\n", consolePassword)
	armbianConf += "PRESET_DEFAULT_REALNAME=\"Intern Kim\"\n"
	armbianConf += "PRESET_USER_SHELL=\"bash\"\n"
	writeContent(armbianConf, "/root/.not_logged_in_yet", "0100600")

	// ── 3. Hostname ──
	fmt.Println("    hostname")
	writeContent("internkim\n", "/etc/hostname", "0100644")

	// ── 3. SSH ──
	if pubKey != "" {
		fmt.Println("    SSH")
		writeContent(pubKey+"\n", "/root/.ssh/authorized_keys", "0100600")
		dbgRun("set_inode_field /root/.ssh mode 040700")
		writeContent("PermitRootLogin yes\nPasswordAuthentication no\n", "/etc/ssh/sshd_config.d/internkim.conf", "0100644")
	}

	// ── 4. Firstboot ──
	fmt.Println("    firstboot")
	firstbootSrc := filepath.Join(stageDir, "internkim-firstboot.sh")
	if _, err := os.Stat(firstbootSrc); err == nil {
		writeFile(firstbootSrc, "/usr/local/bin/internkim-firstboot.sh", "0100755")
	} else {
		fmt.Printf("      WARN: firstboot script not found at %s\n", firstbootSrc)
	}

	// Single trigger: systemd service that waits for network + boot partition
	svcContent := "[Unit]\nDescription=Intern Kim First Boot\nAfter=local-fs.target armbian-firstrun.service armbian-resize-filesystem.service\nWants=local-fs.target\nConditionPathExists=/usr/local/bin/internkim-firstboot.sh\n\n[Service]\nType=oneshot\nExecStartPre=/bin/bash -c 'for i in $$(seq 1 60); do [ -d /boot/firmware/internkim ] && exit 0; sleep 2; done; exit 1'\nExecStart=/usr/local/bin/internkim-firstboot.sh\nRestart=on-failure\nRestartSec=15\nStartLimitIntervalSec=0\nTimeoutStartSec=900\nStandardOutput=journal+console\nStandardError=journal+console\n\n[Install]\nWantedBy=multi-user.target\n"
	writeContent(svcContent, "/etc/systemd/system/internkim-firstboot.service", "0100644")
	mkSymlink("/etc/systemd/system/multi-user.target.wants/internkim-firstboot.service",
		"/etc/systemd/system/internkim-firstboot.service")

	// ── 5b. Pre-downloaded .deb packages ──
	debsTarPath := filepath.Join(filepath.Dir(stageDir), "debs.tar")
	if _, err := os.Stat(debsTarPath); err == nil {
		fmt.Println("    debs.tar (pre-downloaded packages)")
		if err := writeFile(debsTarPath, "/var/cache/internkim/debs.tar", "0100644"); err != nil {
			return fmt.Errorf("inject debs.tar: %w", err)
		}
	} else {
		return fmt.Errorf("debs.tar not found at %s", debsTarPath)
	}

	// ── 6. Cloudflared service ──
	fmt.Println("    cloudflared service")
	cfService := "[Unit]\nDescription=Cloudflare Tunnel\nAfter=network-online.target time-sync.target\nWants=network-online.target time-sync.target\n\n[Service]\nType=simple\nExecStart=/bin/sh -c '/usr/local/bin/cloudflared tunnel run --protocol quic --token \"$(cat /root/.internkim/secrets/tunnel-token)\"'\nRestart=always\nRestartSec=5\n\n[Install]\nWantedBy=multi-user.target\n"
	writeContent(cfService, "/etc/systemd/system/cloudflared.service", "0100644")
	mkSymlink("/etc/systemd/system/multi-user.target.wants/cloudflared.service",
		"/etc/systemd/system/cloudflared.service")

	// ── 7. Watchdog service (runs every boot — ensures Wi-Fi, DNS, SSH, LED) ──
	fmt.Println("    watchdog service")
	watchdogScript := `#!/bin/bash
# Skip if firstboot is still pending
[ -f /usr/local/bin/internkim-firstboot.sh ] && exit 0
# Mask competing network managers (idempotent)
systemctl mask wpa_supplicant.service 2>/dev/null
systemctl mask NetworkManager 2>/dev/null
systemctl mask dhcpcd 2>/dev/null
# Check if Wi-Fi is connected
if ip addr show wlan0 | grep -q 'inet '; then
  : # Wi-Fi OK
else
  # Wi-Fi down — reconnect
  rm -f /run/wpa_supplicant/wlan0
  rm -f /etc/systemd/network/10-netplan-wlan0.network
  systemctl restart wpa_supplicant@wlan0
  systemctl restart systemd-networkd
  for i in $(seq 1 30); do
    ip addr show wlan0 | grep -q 'inet ' && break
    sleep 2
  done
fi
# Ensure DNS
if ! getent hosts google.com >/dev/null 2>&1; then
  DHCP_DNS=$(networkctl status wlan0 2>/dev/null | grep 'DNS:' | awk '{print $2}' | head -1)
  [ -n "$DHCP_DNS" ] && echo "nameserver $DHCP_DNS" > /etc/resolv.conf
  [ -z "$DHCP_DNS" ] && echo "nameserver 8.8.8.8" > /etc/resolv.conf
fi
# Ensure SSH
systemctl start ssh 2>/dev/null || systemctl start sshd 2>/dev/null
# Force NTP sync (RPi5 has no RTC — JWTs fail if clock is stale)
if ! timedatectl show --property=NTPSynchronized --value | grep -q '^yes$'; then
  systemctl restart systemd-timesyncd 2>/dev/null || true
  for i in $(seq 1 15); do
    [ "$(timedatectl show --property=NTPSynchronized --value)" = "yes" ] && break
    sleep 1
  done
fi
# Ensure /etc/hosts maps the hostname (sudo reads it)
HN=$(hostname)
if [ -n "$HN" ] && ! grep -qw "$HN" /etc/hosts; then
  echo "127.0.1.1 $HN" >> /etc/hosts
fi
# Record IP to boot partition
CURRENT_IP=$(ip -4 addr show wlan0 | grep -oP 'inet \K[^/]+' | head -1)
if [ -n "$CURRENT_IP" ]; then
  mountpoint -q /boot/firmware || mount /boot/firmware 2>/dev/null
  mkdir -p /boot/firmware/internkim
  echo "$CURRENT_IP" > /boot/firmware/internkim/board-ip
fi
# LED heartbeat
if [ -f /sys/class/leds/ACT/trigger ]; then
  echo heartbeat > /sys/class/leds/ACT/trigger 2>/dev/null || true
fi
`
	writeContent(watchdogScript, "/usr/local/bin/internkim-watchdog.sh", "0100755")
	watchdogService := "[Unit]\nDescription=Intern Kim Watchdog\n\n[Service]\nType=oneshot\nExecStart=/usr/local/bin/internkim-watchdog.sh\nTimeoutStartSec=120\n"
	writeContent(watchdogService, "/etc/systemd/system/internkim-watchdog.service", "0100644")
	watchdogTimer := "[Unit]\nDescription=Intern Kim Watchdog Timer\n\n[Timer]\nOnBootSec=30\nOnUnitActiveSec=300\n\n[Install]\nWantedBy=timers.target\n"
	writeContent(watchdogTimer, "/etc/systemd/system/internkim-watchdog.timer", "0100644")
	mkSymlink("/etc/systemd/system/timers.target.wants/internkim-watchdog.timer",
		"/etc/systemd/system/internkim-watchdog.timer")

	// ── 8. Expand raw image, update MBR, and write partition back ──
	fmt.Println("    writing partition back")
	newImgSize := partOffset + partSize
	if fi, err := os.Stat(imgRaw); err == nil && fi.Size() < newImgSize {
		os.Truncate(imgRaw, newImgSize)
	}

	// Update MBR partition table with new ext4 size
	newSectors := partSize / 512
	mbrF, err := os.OpenFile(imgRaw, os.O_RDWR, 0)
	if err == nil {
		// Find the ext4 partition entry (type 0x83) and update its sector count
		for i := 0; i < 4; i++ {
			var pbuf [16]byte
			mbrF.Seek(446+int64(i*16), 0)
			mbrF.Read(pbuf[:])
			if pbuf[4] == 0x83 {
				// Update sector count (bytes 12-15, little-endian)
				pbuf[12] = byte(newSectors)
				pbuf[13] = byte(newSectors >> 8)
				pbuf[14] = byte(newSectors >> 16)
				pbuf[15] = byte(newSectors >> 24)
				mbrF.Seek(446+int64(i*16), 0)
				mbrF.Write(pbuf[:])
				break
			}
		}
		mbrF.Close()
	}

	writeBackCmd := exec.Command("dd",
		fmt.Sprintf("if=%s", partFile),
		fmt.Sprintf("of=%s", imgRaw),
		"bs=4096",
		fmt.Sprintf("seek=%d", partOffset/4096),
		"conv=notrunc",
	)
	writeBackCmd.Stderr = os.Stderr
	if err := writeBackCmd.Run(); err != nil {
		return fmt.Errorf("write partition back: %w", err)
	}

	return nil
}

func getKeychainPassword(ssid string) string {
	out, err := exec.Command("security", "find-generic-password",
		"-D", "AirPort network password", "-wa", ssid).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func generateFirstbootScript(isLocalLlamaProvisioned bool) string {
	return buildFirstbootScript(isLocalLlamaProvisioned)
}

type sshBoardConnection struct {
	client *sshClient
}

func (connection sshBoardConnection) Run(command string) string {
	return connection.client.run(command)
}

func (connection sshBoardConnection) SCP(localPath, remotePath string) error {
	return connection.client.scp(localPath, remotePath)
}

type sdStagingTarget struct {
	stagingRoot string
}

func (target sdStagingTarget) RootPath() string {
	return target.stagingRoot
}

func (target sdStagingTarget) WriteFile(stagePath string, data []byte, mode int) error {
	fullPath := filepath.Join(target.stagingRoot, stagePath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, data, os.FileMode(mode))
}

func findSDStagingRoot() string {
	candidates := []string{
		"/Volumes/RPICFG", "/Volumes/bootfs", "/Volumes/boot",
		"/Volumes/NO NAME", "/Volumes/RASPIFIRM",
	}
	for _, volume := range candidates {
		stagingDirectory := filepath.Join(volume, "internkim")
		if info, err := os.Stat(stagingDirectory); err == nil && info.IsDir() {
			return stagingDirectory
		}
	}
	return ""
}
