# LicheeRV Nano 초기 셋업 가이드

준비물: LicheeRV Nano 보드, 마이크로 SD 카드

---

## 1. OS 이미지 다운로드 및 SD 카드에 굽기

[GitHub 릴리즈](https://github.com/sipeed/LicheeRV-Nano-Build/releases)에서 Buildroot 이미지를 다운로드한다. (`20241021` 또는 `20250114` 버전 권장)

```bash
# SD 카드 경로 확인
diskutil list
# /dev/disk2 같은 형태로 표시됨. 반드시 SD 카드인지 확인할 것

# SD 카드 언마운트 (경로는 본인 것으로 변경)
diskutil unmountDisk /dev/disk2

# 압축 해제 없이 바로 굽기
xzcat "이미지파일.img.xz" | sudo dd of=/dev/rdisk2 bs=4m status=progress

# 이미 압축 해제된 경우
sudo dd if="이미지파일.img" of=/dev/rdisk2 bs=4m status=progress
```

> `/dev/rdisk2`처럼 `r`을 붙인 raw 디바이스를 사용하면 훨씬 빠르다.
> SD 카드는 MBR(msdos) 파티션 테이블 기준으로 전체가 미할당 상태여야 한다.

---

## 2. 부팅 및 USB 연결

SD 카드를 보드에 삽입하고 **USB-C 데이터 케이블**로 컴퓨터에 연결한다. (충전 전용 케이블은 안 됨)  
USB가 전원 공급과 네트워크 연결(USB NCM)을 동시에 한다.

- 빨간 LED: 전원 ON
- 파란 LED: 처음에 켜진 후 → 정상 부팅 시 깜빡임 시작
- 파란 LED가 계속 켜져 있으면: SD 카드 문제 또는 이미지 문제

30초 기다린 후 USB를 통해 SSH 접속 가능:

```bash
ssh root@192.168.42.1
# 비밀번호: root
```

---

## 3. Wi-Fi 설정 (자동 스크립트)

USB로 보드에 연결된 상태에서 setup 스크립트를 실행한다.

```bash
# sshpass 설치 (최초 1회)
brew install sshpass

# Wi-Fi 설정 실행
./setup.sh
```

스크립트가 SSID와 비밀번호를 입력받아 보드에 전달하고, 연결 성공 시 Wi-Fi IP를 출력한다.

### 수동 설정 (대안)

USB SSH로 직접 접속하여 설정:

```bash
ssh root@192.168.42.1

# 보드 내부에서 실행
killall wpa_supplicant
cat > /etc/wpa_supplicant.conf <<EOF
ctrl_interface=/var/run/wpa_supplicant
ap_scan=1
network={
  ssid="공유기이름"
  scan_ssid=1
  key_mgmt=WPA-PSK
  psk="비밀번호"
}
EOF
wpa_supplicant -i wlan0 -c /etc/wpa_supplicant.conf -B
udhcpc -i wlan0
```

---

## 5. (대안) UART로 접속 — Ethernet/Wi-Fi 없는 경우

USB-to-TTL 어댑터를 아래 핀에 연결:

| 보드 핀 | 어댑터 |
|---------|--------|
| A17 (RX) | TX |
| A16 (TX) | RX |
| GND | GND |

```bash
# 포트 확인
ls /dev/tty.usbserial-*

screen /dev/tty.usbserial-XXXX 115200
```

---

## 6. 스왑 메모리 설정

RAM이 256MB이고 그 중 128MB는 카메라/디스플레이용으로 예약되어 있어 스왑이 필요하다.

```bash
fallocate -l 1G /swapfile
chmod 600 /swapfile
mkswap /swapfile
swapon /swapfile

# 재부팅 후에도 자동 활성화
echo '/swapfile none swap sw 0 0' | tee -a /etc/fstab
```

---

## 7. 필수 라이브러리 설치

TDL SDK / OpenCV Mobile 사용 시 올바른 버전의 라이브러리가 필요하다.

```bash
# PC에서 보드로 전송
scp required_libs.zip root@192.168.X.X:/root

# 보드에서 압축 해제
cd /root
unzip required_libs.zip

# LD_LIBRARY_PATH 설정 (영구 적용)
echo "export LD_LIBRARY_PATH=/root/libs_patch/lib:/root/libs_patch/middleware_v2:/root/libs_patch/middleware_v2_3rd:/root/libs_patch/tpu_sdk_libs:/root/libs_patch:/root/libs_patch/opencv" | tee -a /etc/profile
```

라이브러리 버전이 맞지 않으면 TDL SDK가 매우 느리게 동작하거나 모델 추론이 실패할 수 있다.

---

## 참고

- [공식 문서 (영문)](https://wiki.sipeed.com/hardware/en/lichee/RV_Nano/1_intro.html)
- [공식 문서 (한문, 더 상세)](https://wiki.sipeed.com/hardware/zh/lichee/RV_Nano/1_intro.html)
- [Buildroot 소스 저장소](https://github.com/sipeed/LicheeRV-Nano-Build)
- [예제 코드 저장소](https://github.com/ZhdanovSt/LicheeRV-Nano-Samples) *(원문 저자)*
- [Milk-V Duo 256 문서](https://milkv.io/docs/duo/getting-started/duo256m) *(같은 SoC, 참고용)*
