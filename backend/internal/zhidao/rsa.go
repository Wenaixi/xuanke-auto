package zhidao

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
)

// 登录页内嵌的 RSA 公钥（DER base64，1024 位）
const pubKeyB64 = "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCWuhgriWbHIbPCQyHmablwQSyItcLyKlQU/0ydXkvU4KJtEExNmuXS0xdoVLBRGxNO5f2u2MkNGzJrFhSpVL68Qc0knhWofzs+BdtpSF4nMi7BteOvOKi0OkvhhCBcHL71Vk8UXOsaKDZkZ3lCBVQHpSA4+s6pi9xIeF93jz6pGwIDAQAB"

// encryptIdentification 复刻前端：账号密码 JSON 序列化后用 RSA 公钥
// PKCS1 v1.5 加密，输出 base64。
func encryptIdentification(account, password string) (string, error) {
	der, err := base64.StdEncoding.DecodeString(pubKeyB64)
	if err != nil {
		return "", err
	}
	pubAny, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return "", err
	}
	rsaPub, ok := pubAny.(*rsa.PublicKey)
	if !ok {
		return "", errors.New("公钥不是 RSA 公钥")
	}
	plain, _ := json.Marshal(map[string]string{"userName": account, "password": password})
	ct, err := rsa.EncryptPKCS1v15(rand.Reader, rsaPub, plain)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ct), nil
}
