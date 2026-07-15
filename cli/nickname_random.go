package main

import (
	"crypto/rand"
	"math/big"
)

var nicknameAdjectives = []string{"Cozy", "Tiny", "Mossy", "Sleepy", "Spicy", "Nimble", "Wobbly", "Sunny", "Turbo", "Quiet"}
var nicknameAnimals = []string{"Otter", "Badger", "Gecko", "Panda", "Moth", "Yak", "Kiwi", "Frog", "Crab", "Bison"}

func randomFunnyNickname() string {
	return sanitizeNickname(randomChoice(nicknameAdjectives) + randomChoice(nicknameAnimals))
}

func randomChoice(values []string) string {
	if len(values) == 0 {
		return ""
	}
	index, err := rand.Int(rand.Reader, big.NewInt(int64(len(values))))
	if err != nil {
		return values[0]
	}
	return values[index.Int64()]
}
