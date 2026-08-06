package handler

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func (h *MetadataHandler) BatchRename(c *gin.Context) {
	var body struct {
		Items []struct {
			ComicID  string `json:"comicId"`
			NewTitle string `json:"newTitle"`
		} `json:"items"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.Items) == 0 {
		c.JSON(400, gin.H{"error": "items array required"})
		return
	}

	success := 0
	failed := 0
	var results []gin.H

	for _, item := range body.Items {
		newTitle := strings.TrimSpace(item.NewTitle)
		if newTitle == "" {
			failed++
			results = append(results, gin.H{"comicId": item.ComicID, "status": "skipped", "message": "empty title"})
			continue
		}

		err := store.UpdateComicFields(item.ComicID, map[string]interface{}{
			"title": newTitle,
		})
		if err != nil {
			failed++
			results = append(results, gin.H{"comicId": item.ComicID, "status": "failed", "message": err.Error()})
		} else {
			success++
			results = append(results, gin.H{"comicId": item.ComicID, "status": "success", "newTitle": newTitle})
		}
	}

	c.JSON(200, gin.H{
		"success": success,
		"failed":  failed,
		"total":   len(body.Items),
		"results": results,
	})
}

// POST /api/metadata/ai-rename — AI 智能批量命名
func (h *MetadataHandler) AIRename(c *gin.Context) {
	var body struct {
		Items []struct {
			ComicID  string `json:"comicId"`
			Filename string `json:"filename"`
			Title    string `json:"title"`
		} `json:"items"`
		Prompt string `json:"prompt"` // 用户的命名需求描述
		Lang   string `json:"lang"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.Items) == 0 {
		c.JSON(400, gin.H{"error": "items array and prompt required"})
		return
	}
	if body.Lang == "" {
		body.Lang = "zh"
	}

	aiCfg := service.LoadAIConfig()
	if !aiCfg.EnableCloudAI || aiCfg.CloudAPIKey == "" {
		c.JSON(400, gin.H{"error": "AI not configured"})
		return
	}

	// 构建待命名列表
	var itemList string
	for i, item := range body.Items {
		itemList += fmt.Sprintf("%d. filename=\"%s\", current_title=\"%s\"\n", i+1, item.Filename, item.Title)
	}

	langName := "中文"
	if body.Lang == "en" {
		langName = "English"
	}

	systemPrompt := fmt.Sprintf(`你是一个专业的漫画/小说命名助手。根据用户的命名需求，为每本书生成合适的名称。

规则：
- 根据用户的命名需求描述来生成新名称
- 如果用户没有特殊要求，则从文件名中智能提取出清晰美观的书名
- 去除文件名中的方括号标记、版本号、扫描组名、文件扩展名等杂项
- 保留核心作品名称、作者等关键信息
- 输出语言为%s
- 返回JSON数组格式，每项包含 index(从1开始) 和 newTitle 字段
- 只返回JSON数组，不要其他内容`, langName)

	userPrompt := fmt.Sprintf(`命名需求：%s

待命名的书籍列表：
%s

请为以上每本书生成新名称，返回JSON数组格式：
[{"index": 1, "newTitle": "新名称"}, ...]`, body.Prompt, itemList)

	content, err := service.CallCloudLLM(aiCfg, systemPrompt, userPrompt, &service.LLMCallOptions{
		Scenario:  "rename",
		MaxTokens: 2000,
	})
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("AI call failed: %v", err)})
		return
	}

	// 清理 markdown 代码块
	content = strings.ReplaceAll(content, "```json", "")
	content = strings.ReplaceAll(content, "```", "")
	content = strings.TrimSpace(content)

	// 提取 JSON 数组
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start >= 0 && end > start {
		content = content[start : end+1]
	}

	var aiResults []struct {
		Index    int    `json:"index"`
		NewTitle string `json:"newTitle"`
	}
	if err := json.Unmarshal([]byte(content), &aiResults); err != nil {
		c.JSON(500, gin.H{"error": "Failed to parse AI response", "raw": content})
		return
	}

	// 映射回原始项
	type RenameResult struct {
		ComicID  string `json:"comicId"`
		Filename string `json:"filename"`
		OldTitle string `json:"oldTitle"`
		NewTitle string `json:"newTitle"`
	}
	var results []RenameResult
	for _, ar := range aiResults {
		idx := ar.Index - 1 // 转换为0-based索引
		if idx >= 0 && idx < len(body.Items) {
			results = append(results, RenameResult{
				ComicID:  body.Items[idx].ComicID,
				Filename: body.Items[idx].Filename,
				OldTitle: body.Items[idx].Title,
				NewTitle: ar.NewTitle,
			})
		}
	}

	c.JSON(200, gin.H{"results": results})
}

// POST /api/metadata/ai-chat — 刮削助手 AI 聊天 (SSE 流式)
// 支持自然语言对话 + 智能指令识别，可控制刮削操作
func (h *MetadataHandler) AIChat(c *gin.Context) {
	var body struct {
		Question string                 `json:"question"`
		History  []service.ChatMessage  `json:"history"`
		Context  map[string]interface{} `json:"context"` // 前端状态上下文
		Lang     string                 `json:"lang"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Question == "" {
		c.JSON(400, gin.H{"error": "question is required"})
		return
	}
	if body.Lang == "" {
		body.Lang = "zh"
	}

	cfg := service.LoadAIConfig()
	if !cfg.EnableCloudAI || cfg.CloudAPIKey == "" {
		c.JSON(400, gin.H{"error": "AI not configured"})
		return
	}

	langName := "中文"
	if body.Lang == "en" {
		langName = "English"
	}

	// 构建上下文信息
	contextJSON, _ := json.Marshal(body.Context)
	contextStr := string(contextJSON)
	if len(contextStr) > 3000 {
		contextStr = contextStr[:3000] + "..."
	}

	systemPrompt := fmt.Sprintf(`你是一个专业的元数据刮削管理助手。你正在帮助用户管理他们的漫画和小说书库的元数据。

## 你的能力：
1. 回答关于元数据刮削的问题（什么是刮削、如何使用、最佳实践等）
2. 帮助用户理解当前的刮削状态和统计信息
3. 通过指令控制刮削操作（需要输出特殊的 JSON 指令块）
4. 提供书库管理建议

## 指令系统：
当用户要求你执行操作时，在回复文本之后，**另起一行**输出特殊标记（注意结尾用 ]] 双方括号关闭）：
<<COMMAND:{"action":"动作名","params":{"参数":"值"}}>>

可用的指令：
- scrape_selected: 刮削选中的项目
- scrape_all: 批量刮削。params.mode = "missing"(仅缺失) 或 "all"(全部)
- set_mode: 设置刮削模式。params.mode = "standard"(标准) 或 "ai"(AI智能)
- select_all: 全选当前页
- deselect_all: 取消全选
- filter: 筛选。params.filter = "all" / "missing" / "with"
- search: 搜索。params.query = 搜索关键词
- enter_batch_edit: 进入批量编辑模式
- stop_scraping: 停止当前刮削
- refresh: 刷新统计和列表
- clear_metadata: 清除选中项的元数据

## 当前书库状态：
%s

## 规则：
- 使用%s回复
- 简洁专业，1-3 句话即可，除非需要更详细的说明
- 如果用户的请求不明确，先确认再执行
- 对危险操作（如清除元数据、全部重刮）要先警告用户
- 如果不需要执行指令，就正常对话即可，不要输出指令标记
- 友好且专业，像一个经验丰富的书库管理员`, contextStr, langName)

	// 构建用户消息
	fullUserMsg := body.Question

	// 构建历史消息
	if len(body.History) > 0 {
		recent := body.History
		if len(recent) > 12 {
			recent = recent[len(recent)-12:]
		}
		var historyText strings.Builder
		historyText.WriteString("[对话历史]\n")
		for _, msg := range recent {
			if msg.Role == "user" {
				historyText.WriteString(fmt.Sprintf("用户: %s\n", msg.Content))
			} else if msg.Role == "assistant" {
				// 移除指令标记避免混淆
				content := msg.Content
				if idx := strings.Index(content, "<<COMMAND:"); idx >= 0 {
					content = strings.TrimSpace(content[:idx])
				}
				historyText.WriteString(fmt.Sprintf("助手: %s\n", content))
			}
		}
		historyText.WriteString("\n")
		fullUserMsg = historyText.String() + "[用户提问]\n" + body.Question
	}

	// 设置 SSE 响应头
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	// 发送初始化事件（确保代理/中间件开始转发SSE流）
	initData, _ := json.Marshal(gin.H{"type": "init"})
	fmt.Fprintf(c.Writer, "data: %s\n\n", initData)
	c.Writer.Flush()

	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2000
	}

	var fullResponse strings.Builder

	err := service.CallCloudLLMStream(cfg, systemPrompt, fullUserMsg, &service.LLMCallOptions{
		Scenario:  "scraper_chat",
		MaxTokens: maxTokens,
	}, func(chunk service.StreamChunk) bool {
		if chunk.Error != "" {
			data, _ := json.Marshal(gin.H{"error": chunk.Error, "done": true})
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()
			return false
		}

		if chunk.Content != "" {
			fullResponse.WriteString(chunk.Content)

			// 检查是否有指令标记（使用 <<COMMAND:...>> 避免 ] 冲突）
			current := fullResponse.String()
			cmdPrefix := "<<COMMAND:"
			cmdSuffix := ">>"
			if cmdIdx := strings.Index(current, cmdPrefix); cmdIdx >= 0 {
				afterPrefix := current[cmdIdx+len(cmdPrefix):]
				if endIdx := strings.Index(afterPrefix, cmdSuffix); endIdx >= 0 {
					cmdStr := afterPrefix[:endIdx]
					// 发送指令事件
					var cmdObj map[string]interface{}
					if err := json.Unmarshal([]byte(cmdStr), &cmdObj); err == nil {
						cmdData, _ := json.Marshal(gin.H{"command": cmdObj})
						fmt.Fprintf(c.Writer, "data: %s\n\n", cmdData)
						c.Writer.Flush()
					}
					// 发送指令之后的剩余文本
					afterCmd := strings.TrimSpace(afterPrefix[endIdx+len(cmdSuffix):])
					if afterCmd != "" {
						data, _ := json.Marshal(gin.H{"content": afterCmd})
						fmt.Fprintf(c.Writer, "data: %s\n\n", data)
						c.Writer.Flush()
					}
					// 重置 fullResponse 为只包含指令之前的文本
					beforeCmd := current[:cmdIdx]
					fullResponse.Reset()
					fullResponse.WriteString(beforeCmd)
					return true
				}
				// 如果还没看到 >>，说明指令还没传输完，暂不输出
				return true
			}

			// 正常的文本块（没有指令标记开头）
			data, _ := json.Marshal(gin.H{"content": chunk.Content})
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()
		}

		if chunk.Done {
			// 最后检查一次是否有未处理的指令
			final := fullResponse.String()
			cmdPrefix := "<<COMMAND:"
			cmdSuffix := ">>"
			if cmdIdx := strings.Index(final, cmdPrefix); cmdIdx >= 0 {
				afterPrefix := final[cmdIdx+len(cmdPrefix):]
				if endIdx := strings.Index(afterPrefix, cmdSuffix); endIdx >= 0 {
					cmdStr := afterPrefix[:endIdx]
					var cmdObj map[string]interface{}
					if err := json.Unmarshal([]byte(cmdStr), &cmdObj); err == nil {
						cmdData, _ := json.Marshal(gin.H{"command": cmdObj})
						fmt.Fprintf(c.Writer, "data: %s\n\n", cmdData)
						c.Writer.Flush()
					}
				}
			}

			doneData, _ := json.Marshal(gin.H{"done": true})
			fmt.Fprintf(c.Writer, "data: %s\n\n", doneData)
			c.Writer.Flush()
			return false
		}

		return true
	})

	if err != nil {
		errData, _ := json.Marshal(gin.H{"error": err.Error(), "done": true})
		fmt.Fprintf(c.Writer, "data: %s\n\n", errData)
		c.Writer.Flush()
	}
}
