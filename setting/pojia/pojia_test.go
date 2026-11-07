package pojia

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestCatalogAndMatch(t *testing.T) {
	cat := Catalog()
	if len(cat) != 42 {
		t.Fatalf("expected 42 profiles, got %d", len(cat))
	}
	// 精确匹配
	inst := Instruction([]string{"hanshuang-deepseek-flash-v1"}, "deepseek-flash", false)
	if inst == "" {
		t.Fatal("hanshuang profile should match deepseek-flash")
	}
	if !strings.Contains(inst, "# DeepSeek API request adapter") {
		t.Fatal("deepseek family must append api-adapter")
	}
	// 不命中 + 无兜底
	if Instruction([]string{"hanshuang-deepseek-flash-v1"}, "gpt-6", false) != "" {
		t.Fatal("unmatched model must not inject")
	}
	// 兜底
	if Instruction(nil, "anything", true) == "" {
		t.Fatal("default fallback should return default.md")
	}
	// 渠道应用
	s := &dto.ChannelSettings{PojiaEnabled: true, PojiaProfiles: []string{"gpt-5.6-sol-v45"}}
	ApplyToChannelSetting(s, "gpt-5.6-sol")
	if s.SystemPrompt == "" || !s.SystemPromptOverride {
		t.Fatal("pojia should set SystemPrompt+Override on match")
	}
	s2 := &dto.ChannelSettings{PojiaEnabled: true, PojiaProfiles: []string{"gpt-5.6-sol-v45"}, SystemPrompt: "keep me"}
	ApplyToChannelSetting(s2, "unmatched")
	if s2.SystemPrompt != "keep me" {
		t.Fatal("unmatched must not clobber existing system prompt")
	}
}
