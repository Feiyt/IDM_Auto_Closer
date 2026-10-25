package main

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

const (
	hkeyCurrentUser  = 0x80000001
	keyRead          = 0x20019
	keySetValue      = 0x0002
	regEnumValue     = 259
	startupValueName = "IDM Auto Closer"
)

var (
	modAdvapi32      = syscall.NewLazyDLL("advapi32.dll")
	modUser32        = syscall.NewLazyDLL("user32.dll")
	regOpenKeyExW    = modAdvapi32.NewProc("RegOpenKeyExW")
	regEnumValueW    = modAdvapi32.NewProc("RegEnumValueW")
	regDeleteValueW  = modAdvapi32.NewProc("RegDeleteValueW")
	regCloseKey      = modAdvapi32.NewProc("RegCloseKey")
	messageBoxW      = modUser32.NewProc("MessageBoxW")
)

func main() {
	message := "清理完成：已移除 IDM Auto Closer 的开机启动项及旧版本启动项。\n\n运行中的程序占用信息由 Windows 在程序退出后自动释放，未修改其他软件配置。"
	if err := removeStartupEntries(); err != nil {
		message = fmt.Sprintf("清理失败：%v\n\n未修改其他软件配置。", err)
	}
	showMessage("IDM Auto Closer 清除程序", message)
}

func removeStartupEntries() error {
	keyPath, err := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Run`)
	if err != nil {
		return fmt.Errorf("转换注册表路径失败：%w", err)
	}

	var keyHandle uintptr
	ret, _, _ := regOpenKeyExW.Call(
		uintptr(hkeyCurrentUser),
		uintptr(unsafe.Pointer(keyPath)),
		0,
		uintptr(keyRead|keySetValue),
		uintptr(unsafe.Pointer(&keyHandle)),
	)
	if ret != 0 {
		return fmt.Errorf("打开启动项注册表失败，错误码：%d", ret)
	}
	defer regCloseKey.Call(keyHandle)

	for index := uint32(0); ; {
		valueName := make([]uint16, 256)
		valueNameLength := uint32(len(valueName))
		valueType := uint32(0)
		valueData := make([]byte, 4096)
		valueDataLength := uint32(len(valueData))
		ret, _, _ := regEnumValueW.Call(
			keyHandle,
			uintptr(index),
			uintptr(unsafe.Pointer(&valueName[0])),
			uintptr(unsafe.Pointer(&valueNameLength)),
			0,
			uintptr(unsafe.Pointer(&valueType)),
			uintptr(unsafe.Pointer(&valueData[0])),
			uintptr(unsafe.Pointer(&valueDataLength)),
		)
		if ret == regEnumValue {
			return nil
		}
		if ret != 0 {
			return fmt.Errorf("读取启动项失败，错误码：%d", ret)
		}

		name := syscall.UTF16ToString(valueName[:valueNameLength])
		if isOwnedStartupValue(name) {
			namePtr, err := syscall.UTF16PtrFromString(name)
			if err != nil {
				return fmt.Errorf("转换启动项名称失败：%w", err)
			}
			ret, _, _ = regDeleteValueW.Call(keyHandle, uintptr(unsafe.Pointer(namePtr)))
			if ret != 0 {
				return fmt.Errorf("删除启动项失败，错误码：%d", ret)
			}
			continue
		}
		index++
	}
}

func isOwnedStartupValue(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	return normalized == strings.ToLower(startupValueName) ||
		normalized == "idm_auto_closer" ||
		strings.HasPrefix(normalized, "idm auto closer ") ||
		strings.HasPrefix(normalized, "idm_auto_closer-") ||
		strings.HasPrefix(normalized, "idm_auto_closer ")
}

func showMessage(title, message string) {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	messagePtr, _ := syscall.UTF16PtrFromString(message)
	messageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(messagePtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		0x00000040,
	)
}
