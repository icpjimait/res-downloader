package core

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var (
	// 51cg1 等站点使用的默认 AES-128-CBC 密钥与 IV
	DefaultImageAESKey = []byte("f5d965df75336270")
	DefaultImageAESIV  = []byte("97b60394abc2fbe1")
	EncryptedImgPrefix = []byte("{s}g=-")
)

// PKCS7Unpad 移除 PKCS#7 填充
func PKCS7Unpad(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, errors.New("empty data")
	}
	padding := int(data[length-1])
	if padding == 0 || padding > aes.BlockSize || padding > length {
		return nil, errors.New("invalid padding")
	}
	for i := length - padding; i < length; i++ {
		if data[i] != byte(padding) {
			return nil, errors.New("invalid padding byte")
		}
	}
	return data[:length-padding], nil
}

// IsImageMagic 检测数据是否为常见图片的魔数
func IsImageMagic(data []byte) bool {
	if bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")) {
		return true
	}
	if bytes.HasPrefix(data, []byte("\xFF\xD8\xFF")) {
		return true
	}
	if bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		return true
	}
	if len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && string(data[8:12]) == "WEBP" {
		return true
	}
	if bytes.HasPrefix(data, []byte("BM")) {
		return true
	}
	return false
}

// DetectImageType 根据文件魔数检测真实图片格式与扩展名
func DetectImageType(data []byte) (string, string) {
	if bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")) {
		return "image/gif", ".gif"
	}
	if bytes.HasPrefix(data, []byte("\xFF\xD8\xFF")) {
		return "image/jpeg", ".jpg"
	}
	if bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		return "image/png", ".png"
	}
	if len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && string(data[8:12]) == "WEBP" {
		return "image/webp", ".webp"
	}
	if bytes.HasPrefix(data, []byte("BM")) {
		return "image/bmp", ".bmp"
	}
	return "image/png", ".png"
}

// DecryptImageBytes 检测数据是否为加密图片（带 {s}g=- 前缀或纯密文），若是则执行 AES-128-CBC 解密
// 返回 (解密后数据, 是否为加密图, MIME类型, 扩展名, 错误)
func DecryptImageBytes(data []byte) ([]byte, bool, string, string, error) {
	if len(data) < 16 {
		return data, false, "", "", nil
	}

	var ciphertext []byte
	if bytes.HasPrefix(data, EncryptedImgPrefix) {
		ciphertext = data[len(EncryptedImgPrefix):]
	} else if len(data)%aes.BlockSize == 0 && !IsImageMagic(data) {
		// 尝试测试解密首个 16 字节分组
		block, err := aes.NewCipher(DefaultImageAESKey)
		if err == nil {
			mode := cipher.NewCBCDecrypter(block, DefaultImageAESIV)
			testFirst := make([]byte, 16)
			mode.CryptBlocks(testFirst, data[:16])
			if IsImageMagic(testFirst) {
				ciphertext = data
			}
		}
	}

	if len(ciphertext) == 0 {
		return data, false, "", "", nil
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return data, true, "", "", errors.New("ciphertext length not multiple of 16")
	}

	block, err := aes.NewCipher(DefaultImageAESKey)
	if err != nil {
		return data, true, "", "", err
	}

	mode := cipher.NewCBCDecrypter(block, DefaultImageAESIV)
	plain := make([]byte, len(ciphertext))
	mode.CryptBlocks(plain, ciphertext)

	unpadded, err := PKCS7Unpad(plain)
	if err == nil {
		plain = unpadded
	}

	mimeType, ext := DetectImageType(plain)
	return plain, true, mimeType, ext, nil
}

// DecryptFileOnDisk 检查磁盘上的文件是否为加密图片，如果是则原地解密并修正扩展名
// 返回最终解密后的文件路径及错误
func DecryptFileOnDisk(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return filePath, err
	}

	decrypted, isEncrypted, _, ext, err := DecryptImageBytes(data)
	if !isEncrypted || err != nil {
		return filePath, err
	}

	newPath := filePath
	currentExt := strings.ToLower(filepath.Ext(filePath))
	if ext != "" && currentExt != ext {
		baseWithoutExt := strings.TrimSuffix(filePath, filepath.Ext(filePath))
		newPath = baseWithoutExt + ext
	}

	if err := os.WriteFile(newPath, decrypted, 0644); err != nil {
		return filePath, err
	}
	if newPath != filePath {
		_ = os.Remove(filePath)
	}

	return newPath, nil
}
