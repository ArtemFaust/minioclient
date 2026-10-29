package utils

import (
	"strings"

	"github.com/AlexanderGrom/componenta/crypt"
)

// Метод шифрования строки
func Encoder(encodestring string, salt string) ([]byte, error) {
	encode, e := crypt.Encrypt(encodestring, strings.TrimSuffix(string(salt), "\n")) // Шифруем AES с солью
	if e != nil {
		return nil, e
	}

	return []byte(encode), nil
}

// Метод дешифрования строки
func Decoder(decodestring string, salt string) ([]byte, error) {
	p, e := crypt.Decrypt(decodestring, strings.TrimSuffix(string(salt), "\n")) // Дешифруем AES с солью
	if e != nil {
		return nil, e
	}

	return []byte(p), e
}
