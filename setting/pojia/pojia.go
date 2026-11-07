// Package pojia 内嵌 ZakoAI Proxy 的破甲 profile 目录，按渠道配置把
// jailbreak instruction 注入到上游请求的 system / instructions / messages。
// 逻辑对齐 zako 的 src/profiles.rs：profile 按精确模型名匹配，
// deepseek 家族追加 dsh-purge api-adapter，default_strategy 兜底 default.md。
package pojia

import (
	"embed"
	"encoding/json"
	"sort"
	"sync"

	"github.com/QuantumNous/new-api/dto"
)

//go:embed profiles
var profilesFS embed.FS

type profileEntry struct {
	Id          string   `json:"id"`
	Name        string   `json:"name"`
	Models      []string `json:"models"`
	Instruction string   `json:"instruction"`
	ModelFamily string   `json:"model_family"`
}

// Profile 是暴露给管理端/API 的目录条目（Instruction 不随目录接口下发）。
type Profile struct {
	Id          string   `json:"id"`
	Name        string   `json:"name"`
	Models      []string `json:"models"`
	ModelFamily string   `json:"model_family"`
}

var modelFamilies = []string{
	"gpt-5.6", "gpt-6.1", "gpt-6", "claude", "grok", "gemini", "deepseek", "glm", "kimi",
}

// ModelFamily 复刻 zako model_family：精确相等或以 "家族名-" 为前缀。
func ModelFamily(model string) string {
	for _, family := range modelFamilies {
		if model == family {
			return family
		}
		if len(model) > len(family) && model[:len(family)] == family && model[len(family)] == '-' {
			return family
		}
	}
	return ""
}

var (
	catalogOnce sync.Once
	catalog     []*profileEntry
	byId        map[string]*profileEntry
)

func mustReadFile(path string) string {
	data, err := profilesFS.ReadFile(path)
	if err != nil {
		panic("pojia: missing embedded profile asset " + path + ": " + err.Error())
	}
	return string(data)
}

func loadIndex(path string) []*profileEntry {
	var entries []*profileEntry
	if err := json.Unmarshal([]byte(mustReadFile(path)), &entries); err != nil {
		panic("pojia: invalid embedded profile index " + path + ": " + err.Error())
	}
	return entries
}

var deepseekAPIAdapter = mustReadFile("profiles/dsh-purge/api-adapter.md")

func buildCatalog() {
	entries := loadIndex("profiles/index.json")

	// coldcoffee 包：instruction 按 model_family 取对应 md
	coldcoffee := loadIndex("profiles/coldcoffee/index.json")
	coldcoffeeFiles := map[string]string{
		"claude":   "profiles/coldcoffee/claude.md",
		"grok":     "profiles/coldcoffee/grok.md",
		"gemini":   "profiles/coldcoffee/gemini.md",
		"deepseek": "profiles/coldcoffee/deepseek.md",
		"glm":      "profiles/coldcoffee/glm53.md",
	}
	for _, p := range coldcoffee {
		file, ok := coldcoffeeFiles[p.ModelFamily]
		if !ok {
			panic("pojia: unsupported coldcoffee model family " + p.ModelFamily)
		}
		p.Instruction = mustReadFile(file)
		if p.Id == "coldcoffee-deepseek-v4.1-flash-v1" {
			// 历史 ID 保留，展示名与 hanshuang 版本一致，并额外覆盖 deepseek-4.1-flash
			p.Name = "hanshuang · deepseek-v4.1-flash"
			p.Models = append(p.Models, "deepseek-4.1-flash")
			p.Instruction = mustReadFile("profiles/hanshuang/system.md")
		}
		entries = append(entries, p)
	}

	// hanshuang 包：统一 system.md
	for _, p := range loadIndex("profiles/hanshuang/index.json") {
		p.Instruction = mustReadFile("profiles/hanshuang/system.md")
		entries = append(entries, p)
	}

	// kimi-break 包：统一 system.md
	for _, p := range loadIndex("profiles/kimi-break/index.json") {
		p.Instruction = mustReadFile("profiles/kimi-break/system.md")
		entries = append(entries, p)
	}

	// ai-jailbreak 包：instruction 按 model_family 取
	aiJailbreakFiles := map[string]string{
		"glm":  "profiles/ai-jailbreak/glm.md",
		"grok": "profiles/ai-jailbreak/grok.md",
	}
	for _, p := range loadIndex("profiles/ai-jailbreak/index.json") {
		file, ok := aiJailbreakFiles[p.ModelFamily]
		if !ok {
			panic("pojia: unsupported ai-jailbreak model family " + p.ModelFamily)
		}
		p.Instruction = mustReadFile(file)
		entries = append(entries, p)
	}

	byId = make(map[string]*profileEntry, len(entries))
	for _, p := range entries {
		if _, dup := byId[p.Id]; dup {
			panic("pojia: duplicate profile id " + p.Id)
		}
		if len(p.Models) == 0 || ModelFamily(p.Models[0]) == "" {
			panic("pojia: profile " + p.Id + " has no supported model family")
		}
		family := ModelFamily(p.Models[0])
		for _, m := range p.Models {
			if ModelFamily(m) != family {
				panic("pojia: profile " + p.Id + " mixes model families")
			}
		}
		if p.Instruction == "" {
			panic("pojia: profile " + p.Id + " has empty instruction")
		}
		if family == "deepseek" {
			p.Instruction += "\n\n" + deepseekAPIAdapter
		}
		p.ModelFamily = family
		byId[p.Id] = p
	}
	catalog = entries
}

func ensureCatalog() {
	catalogOnce.Do(buildCatalog)
}

// Catalog 返回全部 profile 的目录信息（不含 instruction 正文）。
func Catalog() []Profile {
	ensureCatalog()
	out := make([]Profile, 0, len(catalog))
	for _, p := range catalog {
		out = append(out, Profile{
			Id:          p.Id,
			Name:        p.Name,
			Models:      p.Models,
			ModelFamily: p.ModelFamily,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Id < out[j].Id })
	return out
}

// Instruction 按精确模型名在已选 profile 中命中并返回 instruction；
// 无命中且开启 defaultStrategy 时返回内置 default.md。
func Instruction(profileIds []string, model string, defaultStrategy bool) string {
	ensureCatalog()
	for _, id := range profileIds {
		p, ok := byId[id]
		if !ok {
			continue
		}
		for _, m := range p.Models {
			if m == model {
				return p.Instruction
			}
		}
	}
	if defaultStrategy {
		return mustReadFile("profiles/default.md")
	}
	return ""
}

// ApplyToChannelSetting 把命中到的破甲 instruction 写入渠道的
// SystemPrompt/SystemPromptOverride，让 chat/claude/gemini/codex 各
// 协议路径沿用既有注入逻辑。未命中时不动渠道设置。
func ApplyToChannelSetting(setting *dto.ChannelSettings, model string) {
	if setting == nil || !setting.PojiaEnabled {
		return
	}
	instruction := Instruction(setting.PojiaProfiles, model, setting.PojiaFallback)
	if instruction == "" {
		return
	}
	setting.SystemPrompt = instruction
	setting.SystemPromptOverride = true
}
