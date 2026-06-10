package worktreesvc

import (
	crand "crypto/rand"
	"encoding/hex"
	"math/rand/v2"
)

// 「形容词-动物」随机 slug（对齐 paseo 的 mnemonic-id 设计：创建零用户输入、目录名
// 即占位分支名）。词表刻意全小写 ASCII，天然满足 git 分支名与目录名的安全字符集。
var slugAdjectives = []string{
	"agile", "amber", "bold", "brave", "bright", "calm", "clever", "cosmic",
	"crisp", "daring", "deft", "eager", "fancy", "fleet", "gentle", "glad",
	"golden", "happy", "hardy", "humble", "ivory", "jolly", "keen", "kind",
	"lively", "lucid", "lucky", "mellow", "merry", "mighty", "nimble", "noble",
	"polite", "proud", "quick", "quiet", "rapid", "rosy", "shiny", "silent",
	"sleek", "smart", "snappy", "solid", "sturdy", "sunny", "swift", "tidy",
	"vivid", "witty",
}

var slugAnimals = []string{
	"badger", "beaver", "bison", "bobcat", "camel", "civet", "condor", "crane",
	"dingo", "donkey", "falcon", "ferret", "finch", "gecko", "gibbon", "heron",
	"hyena", "ibex", "jackal", "jaguar", "koala", "lemur", "linnet", "lizard",
	"llama", "lynx", "macaw", "marmot", "marten", "mole", "moose", "narwhal",
	"ocelot", "osprey", "otter", "owl", "panda", "pelican", "puffin", "quokka",
	"rabbit", "racoon", "robin", "serval", "shrew", "sloth", "stork", "tapir",
	"toucan", "wombat",
}

// randomSlug 生成一个「形容词-动物」组合（约 2500 种）。
func randomSlug() string {
	return slugAdjectives[rand.IntN(len(slugAdjectives))] + "-" + slugAnimals[rand.IntN(len(slugAnimals))]
}

// randomHexSuffix 返回 4 字符十六进制（slug 多次碰撞后的兜底消歧）。
func randomHexSuffix() string {
	b := make([]byte, 2)
	_, _ = crand.Read(b) // crypto/rand.Read 实践上不失败；失败则零值后缀亦可用
	return hex.EncodeToString(b)
}
