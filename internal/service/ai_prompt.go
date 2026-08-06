package service

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/nowen-reader/nowen-reader/internal/config"
)

// ============================================================
// Phase 2-3: Prompt 模板管理
// ============================================================

// PromptPair 一对 system + user prompt 模板
type PromptPair struct {
	System string `json:"system"`
	User   string `json:"user"`
}

// PromptTemplates 所有可自定义的 prompt 模板
type PromptTemplates struct {
	Summary         PromptPair `json:"summary"`         // AI 摘要生成
	ParseFilename   PromptPair `json:"parseFilename"`   // AI 文件名解析
	SuggestTags     PromptPair `json:"suggestTags"`     // AI 标签建议
	CoverAnalysis   PromptPair `json:"coverAnalysis"`   // Vision 封面分析
	RecommendReason PromptPair `json:"recommendReason"` // 推荐理由生成
	Translate       PromptPair `json:"translate"`       // 元数据翻译
}

func promptTemplatesPath() string {
	return filepath.Join(config.DataDir(), "prompt-templates.json")
}

// LoadPromptTemplates 加载自定义 prompt 模板（为空则使用代码内置默认值）
func LoadPromptTemplates() PromptTemplates {
	var templates PromptTemplates
	data, err := os.ReadFile(promptTemplatesPath())
	if err != nil {
		return templates // 返回空模板，各功能将使用内置默认值
	}
	_ = json.Unmarshal(data, &templates)
	return templates
}

// SavePromptTemplates 保存自定义 prompt 模板
func SavePromptTemplates(templates PromptTemplates) error {
	dir := filepath.Dir(promptTemplatesPath())
	os.MkdirAll(dir, 0755)
	data, _ := json.MarshalIndent(templates, "", "  ")
	return os.WriteFile(promptTemplatesPath(), data, 0644)
}

// ResetPromptTemplates 重置为默认模板（删除自定义文件）
func ResetPromptTemplates() error {
	return os.Remove(promptTemplatesPath())
}

// GetDefaultPromptTemplates 返回内置的默认 prompt 模板（供前端展示参考）
func GetDefaultPromptTemplates() PromptTemplates {
	return PromptTemplates{
		Summary: PromptPair{
			System: "You are a professional {contentType} reviewer and librarian. Based on the given metadata, write an engaging and informative summary/description in {language}.\n\nRequirements:\n- Write 2-4 sentences (80-200 characters for Chinese, 100-300 words for English)\n- Be descriptive and engaging, like a bookstore blurb\n- If the existing description exists, improve and localize it\n- Do NOT add any prefixes, labels, or markdown — return only the pure summary text",
			User:   "Generate a {language} summary for this {contentType} based on the following metadata:\n\n{metadata}",
		},
		ParseFilename: PromptPair{
			System: "You are an expert at parsing manga/comic/novel filenames. Extract structured metadata from complex filename conventions.\n\nRules:\n- \"Group\" refers to scan/translation groups\n- Remove file extensions before parsing\n- Return ONLY a valid JSON object",
			User:   "Parse this filename and extract structured metadata:\n\n\"{filename}\"\n\nReturn a JSON object with fields: title, author, group, language, genre, year, tags",
		},
		SuggestTags: PromptPair{
			System: "You are an expert {contentType} librarian and tagger. Suggest 5-10 relevant tags in {language}.\n\nRequirements:\n- Tags should be concise (1-4 words)\n- Include genre, theme, and mood/style tags\n- Don't repeat existing tags\n- Return ONLY a JSON array of tag strings",
			User:   "Suggest tags for this {contentType}:\n\n{metadata}\n\nReturn a JSON array of tag strings.",
		},
		CoverAnalysis: PromptPair{
			System: "You are an expert cover analyst. Analyze the cover image and return a JSON object with: style, mood, theme, ageRating, colorTone, characters, tags, description, confidence.",
			User:   "Analyze this {contentType} cover image. Return a JSON object.",
		},
		RecommendReason: PromptPair{
			System: "You are a friendly recommendation curator. Generate short, engaging recommendation reasons (1 sentence, max 50 chars for Chinese / 80 chars for English). Return a JSON object mapping IDs to reason strings.",
			User:   "Generate personalized recommendation reasons in {language} for each item:",
		},
		Translate: PromptPair{
			System: "You are a professional translator specializing in manga/comic metadata. Translate the given fields to {language}. Keep proper nouns in their commonly known form.\nRespond ONLY with a valid JSON object.",
			User:   "Translate these metadata fields to {language}:\n\n{fields}\n\nReturn a JSON object with the same keys and translated values.",
		},
	}
}
