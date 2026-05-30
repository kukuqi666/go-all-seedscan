package core

import (
	"crypto/sha256"

	"github.com/tyler-smith/go-bip39"
)

var (
	wordList    []string
	wordListMap map[string]int
)

func InitWordList() {
	wordList = bip39.GetWordList()
	wordListMap = make(map[string]int, len(wordList))
	for i, w := range wordList {
		wordListMap[w] = i
	}
}

func FastChecksumValid(words []string) bool {
	n := len(words)
	if n != 12 && n != 24 {
		return false
	}

	totalBits := n * 11
	entropyBits := totalBits - totalBits/32
	checksumBits := totalBits - entropyBits
	entropyBytes := entropyBits / 8
	totalBytes := (totalBits + 7) / 8

	buf := make([]byte, totalBytes)
	bitPos := 0
	for _, w := range words {
		idx, ok := wordListMap[w]
		if !ok {
			return false
		}
		for i := 10; i >= 0; i-- {
			byteIdx := bitPos / 8
			bitIdx := 7 - (bitPos % 8)
			if idx&(1<<i) != 0 {
				buf[byteIdx] |= 1 << bitIdx
			}
			bitPos++
		}
	}

	entropy := buf[:entropyBytes]
	var checksum byte
	for i := range checksumBits {
		byteIdx := (entropyBytes*8 + i) / 8
		bitIdx := 7 - ((entropyBytes*8 + i) % 8)
		if (buf[byteIdx]>>bitIdx)&1 != 0 {
			checksum |= 1 << (7 - i)
		}
	}

	checksum >>= (8 - checksumBits)
	hash := sha256.Sum256(entropy)
	return (hash[0] >> (8 - checksumBits)) == checksum
}
