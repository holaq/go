package encrypt

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const Base64Table = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

var Base64Coder = base64.NewEncoding(Base64Table)

// Base64.Encode
func EncodeToString(s []byte) string {
	return Base64Coder.EncodeToString(s)
}
func DecodeToBytes(s string) []byte {
	bs, _ := Base64Coder.DecodeString(s)
	return bs
}
func Base64Decode(s string) ([]byte, error) {
	return Base64Coder.DecodeString(s)
}

func PKCS5Padding(cipherText []byte, blockSize int) []byte {
	padding := blockSize - len(cipherText)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(cipherText, padText...)
}
func PKCS5UnPadding(bs []byte) []byte {
	leng := len(bs)
	unpadding := int(bs[leng-1])
	return bs[:(leng - unpadding)]
}
func AesEncode(s string, key string) (result string) {
	defer func() {
		recover()
	}()
	return EncodeToString(AesEncodeBs2Bs([]byte(s), []byte(key)))
}

func AesDecode(s string, key string) (result string) {
	return string(AesDecodeForBytes(s, []byte(key)))
}

func AesDecodeForBytes(s string, keyBytes []byte) (resultBytes []byte) {
	defer func() {
		recover()
	}()
	return AesDecodeBs2Bs(DecodeToBytes(s), keyBytes)
}
func AesDecodeBs2Bs(originBytes []byte, keyBytes []byte) (resultBytes []byte) {
	// defer func() {
	// 	recover()
	// }()

	// block, err := aes.NewCipher(keyBytes)
	// if err != nil {
	// 	return
	// }
	// blockSize := block.BlockSize()
	// blockMode := cipher.NewCBCDecrypter(block, keyBytes[:blockSize])
	// resultBytes = make([]byte, len(originBytes))
	// blockMode.CryptBlocks(resultBytes, originBytes)
	// resultBytes = PKCS5UnPadding(resultBytes)
	return AesDecodeBs2BsWithIV(originBytes, keyBytes, keyBytes)
}

func AesCBCEncode(content, key, iv []byte) (res []byte, e error) {
	defer func() {
		if _e := recover(); _e != nil {
			if e == nil {
				var yes bool
				if e, yes = _e.(error); !yes {
					e = errors.New(fmt.Sprintf("Panic:%v", _e))
				}
			}
		}
	}()

	var block cipher.Block
	block, e = aes.NewCipher(key)
	if e != nil {
		return
	}
	blockSize := block.BlockSize()
	content = PKCS5Padding(content, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, iv[:blockSize])
	res = make([]byte, len(content))
	blockMode.CryptBlocks(res, content)
	return
}

func AesCBCDecode(bs, key, iv []byte) (res []byte, e error) {
	defer func() {
		if _e := recover(); _e != nil {
			if e == nil {
				var yes bool
				if e, yes = _e.(error); !yes {
					e = errors.New(fmt.Sprintf("Panic:%v", _e))
				}
			}
		}
	}()

	var block cipher.Block
	block, e = aes.NewCipher(key)
	if e != nil {
		return
	}
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, iv[:blockSize])
	res = make([]byte, len(bs))
	blockMode.CryptBlocks(res, bs)
	res = PKCS5UnPadding(res)
	return
}

func AesDecodeBs2BsWithIV(originBytes, keyBytes, ivBytes []byte) (resultBytes []byte) {
	defer func() {
		if e := recover(); e != nil {
			fmt.Println("Failed to aes decode:", e)
		}
	}()

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return
	}
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, ivBytes[:blockSize])
	resultBytes = make([]byte, len(originBytes))
	blockMode.CryptBlocks(resultBytes, originBytes)
	resultBytes = PKCS5UnPadding(resultBytes)
	return
}
func AesEncodeBs2Bs(content []byte, key []byte) (result []byte) {
	return AesEncodeBs2BsWithIV(content, key, key)
}

func AesEncodeBs2BsWithIV(content, key, iv []byte) (result []byte) {
	defer func() {
		recover()
	}()

	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}
	blockSize := block.BlockSize()
	content = PKCS5Padding(content, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, iv[:blockSize])
	result = make([]byte, len(content))
	blockMode.CryptBlocks(result, content)
	return
}
func GenerateRSAKeysPKCS8() (pubKeyBytes, priKeyBytes []byte, pubKey *rsa.PublicKey, priKey *rsa.PrivateKey, err error) {
	if priKey, err = rsa.GenerateKey(rand.Reader, 1024); err == nil {
		if priKeyBytes, err = MarshalPKCS8PrivateKey(priKey); err == nil {
			pubKeyBytes, pubKey, err = restorePubKey(priKey)
		}
	}
	return
}

func GenerateRSAKeysPKCS1() (pubKeyBytes, priKeyBytes []byte, pubKey *rsa.PublicKey, priKey *rsa.PrivateKey, err error) {
	if priKey, err = rsa.GenerateKey(rand.Reader, 1024); err == nil {
		priKeyBytes = x509.MarshalPKCS1PrivateKey(priKey)
		pubKeyBytes, pubKey, err = restorePubKey(priKey)
	}
	return
}

func GenerateRSAKeys() (pubKeyBytes, priKeyBytes []byte, pubKey *rsa.PublicKey, priKey *rsa.PrivateKey, err error) {
	return GenerateRSAKeysPKCS1()
}

func MarshalPKCS1PrivateKey(key *rsa.PrivateKey) (priKeyBytes []byte) {
	priKeyBytes = x509.MarshalPKCS1PrivateKey(key)
	return
}

func MarshalPKCS8PrivateKey(key *rsa.PrivateKey) (priKeyBytes []byte, e error) {
	info := struct {
		Version             int
		PrivateKeyAlgorithm []asn1.ObjectIdentifier
		PrivateKey          []byte
	}{}
	info.Version = 0
	info.PrivateKeyAlgorithm = []asn1.ObjectIdentifier{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}}
	info.PrivateKey = x509.MarshalPKCS1PrivateKey(key)

	priKeyBytes, e = asn1.Marshal(info)
	return
}

func RestoreRSAKeys(fromPriKeyBytes []byte) (pubKeyBytes []byte, pubKey *rsa.PublicKey, priKey *rsa.PrivateKey, err error) {
	if priKey, err = x509.ParsePKCS1PrivateKey(fromPriKeyBytes); err == nil {
		pubKeyBytes, pubKey, err = restorePubKey(priKey)
	} else {
		if _pri, _err := x509.ParsePKCS8PrivateKey(fromPriKeyBytes); _err == nil {
			var isOk bool
			if priKey, isOk = _pri.(*rsa.PrivateKey); isOk {
				pubKeyBytes, pubKey, err = restorePubKey(priKey)
			}
		}
	}
	return
}

func ParseRSAPublicKey(fromPubKeyBytes []byte) (pubKey *rsa.PublicKey, err error) {
	var pubI interface{}
	if pubI, err = x509.ParsePKIXPublicKey(fromPubKeyBytes); err == nil {
		var isPubKey bool
		if pubKey, isPubKey = pubI.(*rsa.PublicKey); !isPubKey {
			err = errors.New(fmt.Sprintf("Public key's type is %T", pubI))
		}
	}
	return
}

func restorePubKey(fromPriKey *rsa.PrivateKey) (pubKeyBytes []byte, pubKey *rsa.PublicKey, err error) {
	pubKey = &fromPriKey.PublicKey
	pubKeyBytes, err = x509.MarshalPKIXPublicKey(pubKey)
	return
}

func RSADecode(txt []byte, key *rsa.PrivateKey) (originTxt []byte, err error) {
	if key == nil {
		err = errors.New("private key is null")
		return
	}
	return rsa.DecryptPKCS1v15(rand.Reader, key, txt)
}

func RSAEncode(originTxt []byte, key *rsa.PublicKey) (txt []byte, err error) {
	if key == nil {
		err = errors.New("public key is null")
		return
	}
	return rsa.EncryptPKCS1v15(rand.Reader, key, originTxt)
}

func RSASign(originTxt []byte, priKey *rsa.PrivateKey) (signature []byte, err error) {
	hash := crypto.SHA256
	h := crypto.SHA256.New()
	h.Write(originTxt)
	hashed := h.Sum(nil)
	signature, err = rsa.SignPKCS1v15(rand.Reader, priKey, hash, hashed)
	return
}

func RSAVerify(originTxt, signature []byte, pubKey *rsa.PublicKey) error {
	hash := crypto.SHA256
	h := crypto.SHA256.New()
	h.Write(originTxt)
	hashed := h.Sum(nil)
	return rsa.VerifyPKCS1v15(pubKey, hash, hashed, signature)
}

func MD5(bs []byte) []byte {
	md5Util := md5.New()
	md5Util.Write(bs)
	return md5Util.Sum(nil)
}

func MD5IO(reader io.Reader) []byte {
	md5Util := md5.New()
	io.Copy(md5Util, reader)
	return md5Util.Sum(nil)
}

func SHA1(bs []byte) []byte {
	shaUtil := sha1.New()
	shaUtil.Write(bs)
	return shaUtil.Sum(nil)
}

func SHA1IO(reader io.Reader) []byte {
	shaUtil := sha1.New()
	io.Copy(shaUtil, reader)
	return shaUtil.Sum(nil)
}
