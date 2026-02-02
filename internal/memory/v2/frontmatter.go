// Package v2 提供 Markdown frontmatter 解析功能
package v2

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// FrontmatterDelimiter YAML frontmatter 分隔符
	FrontmatterDelimiter = "---"
)

// FrontmatterParser Markdown frontmatter 解析器
type FrontmatterParser struct{}

// NewFrontmatterParser 创建 frontmatter 解析器
func NewFrontmatterParser() *FrontmatterParser {
	return &FrontmatterParser{}
}

// ParseMemory 从 Markdown 内容解析记忆
func (p *FrontmatterParser) ParseMemory(content []byte) (*Memory, error) {
	fm, body, err := p.Parse(content)
	if err != nil {
		return nil, err
	}

	var memFM MemoryFrontmatter
	if err := yaml.Unmarshal(fm, &memFM); err != nil {
		return nil, fmt.Errorf("解析记忆 frontmatter 失败: %w", err)
	}

	mem := &Memory{
		Content: string(body),
	}
	mem.FromFrontmatter(&memFM)

	return mem, nil
}

// ParseSession 从 Markdown 内容解析会话
func (p *FrontmatterParser) ParseSession(content []byte) (*Session, []byte, error) {
	fm, body, err := p.Parse(content)
	if err != nil {
		return nil, nil, err
	}

	var sessFM SessionFrontmatter
	if err := yaml.Unmarshal(fm, &sessFM); err != nil {
		return nil, nil, fmt.Errorf("解析会话 frontmatter 失败: %w", err)
	}

	sess := &Session{}
	sess.FromFrontmatter(&sessFM)

	return sess, body, nil
}

// Parse 解析 Markdown 文件，分离 frontmatter 和 body
// 返回: (frontmatter YAML bytes, body bytes, error)
func (p *FrontmatterParser) Parse(content []byte) ([]byte, []byte, error) {
	reader := bufio.NewReader(bytes.NewReader(content))

	// 读取第一行，检查是否是 frontmatter 开始
	firstLine, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, nil, fmt.Errorf("读取文件失败: %w", err)
	}

	firstLine = strings.TrimSpace(firstLine)
	if firstLine != FrontmatterDelimiter {
		// 没有 frontmatter，整个内容都是 body
		return nil, content, nil
	}

	// 读取 frontmatter 内容直到遇到结束分隔符
	var frontmatterBuf bytes.Buffer
	var foundEnd bool

	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, nil, fmt.Errorf("读取 frontmatter 失败: %w", err)
		}

		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == FrontmatterDelimiter {
			foundEnd = true
			break
		}

		frontmatterBuf.WriteString(line)

		if err == io.EOF {
			break
		}
	}

	if !foundEnd {
		return nil, nil, fmt.Errorf("frontmatter 未正确结束")
	}

	// 读取剩余内容作为 body
	var bodyBuf bytes.Buffer
	_, err = io.Copy(&bodyBuf, reader)
	if err != nil {
		return nil, nil, fmt.Errorf("读取 body 失败: %w", err)
	}

	// 去除 body 开头的空行
	body := bytes.TrimLeft(bodyBuf.Bytes(), "\n\r")

	return frontmatterBuf.Bytes(), body, nil
}

// SerializeMemory 将记忆序列化为 Markdown 格式
func (p *FrontmatterParser) SerializeMemory(mem *Memory) ([]byte, error) {
	fm := mem.ToFrontmatter()

	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return nil, fmt.Errorf("序列化 frontmatter 失败: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString(FrontmatterDelimiter)
	buf.WriteString("\n")
	buf.Write(fmBytes)
	buf.WriteString(FrontmatterDelimiter)
	buf.WriteString("\n\n")
	buf.WriteString(mem.Content)

	// 确保文件以换行结束
	if !strings.HasSuffix(mem.Content, "\n") {
		buf.WriteString("\n")
	}

	return buf.Bytes(), nil
}

// SerializeSession 将会话序列化为 Markdown 格式（仅 frontmatter）
func (p *FrontmatterParser) SerializeSession(sess *Session, body []byte) ([]byte, error) {
	fm := sess.ToFrontmatter()

	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return nil, fmt.Errorf("序列化会话 frontmatter 失败: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString(FrontmatterDelimiter)
	buf.WriteString("\n")
	buf.Write(fmBytes)
	buf.WriteString(FrontmatterDelimiter)
	buf.WriteString("\n\n")
	buf.Write(body)

	// 确保文件以换行结束
	if len(body) > 0 && !bytes.HasSuffix(body, []byte("\n")) {
		buf.WriteString("\n")
	}

	return buf.Bytes(), nil
}

// UpdateFrontmatter 更新已有 Markdown 文件的 frontmatter（保留 body）
func (p *FrontmatterParser) UpdateFrontmatter(content []byte, newFM interface{}) ([]byte, error) {
	_, body, err := p.Parse(content)
	if err != nil {
		return nil, err
	}

	fmBytes, err := yaml.Marshal(newFM)
	if err != nil {
		return nil, fmt.Errorf("序列化 frontmatter 失败: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString(FrontmatterDelimiter)
	buf.WriteString("\n")
	buf.Write(fmBytes)
	buf.WriteString(FrontmatterDelimiter)
	buf.WriteString("\n\n")
	buf.Write(body)

	return buf.Bytes(), nil
}

// CalculateContentHash 计算内容哈希（SHA256）
func CalculateContentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// CalculateMemoryHash 计算记忆内容哈希（仅计算 body 部分）
func (p *FrontmatterParser) CalculateMemoryHash(content []byte) (string, error) {
	_, body, err := p.Parse(content)
	if err != nil {
		return "", err
	}
	return CalculateContentHash(body), nil
}

// ExtractTitle 从内容中提取标题（取第一行非空行）
func ExtractTitle(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			// 移除 Markdown 标题标记
			line = strings.TrimLeft(line, "# ")
			if len(line) > 50 {
				line = line[:50] + "..."
			}
			return line
		}
	}
	return "无标题"
}

// ExtractKeywords 从内容中提取关键词（简单实现）
func ExtractKeywords(content string, maxKeywords int) []string {
	// 移除 Markdown 格式标记
	content = strings.ReplaceAll(content, "#", "")
	content = strings.ReplaceAll(content, "*", "")
	content = strings.ReplaceAll(content, "`", "")
	content = strings.ReplaceAll(content, "_", "")

	// 分词
	words := strings.Fields(content)

	// 统计词频
	wordCount := make(map[string]int)
	for _, word := range words {
		word = strings.ToLower(strings.Trim(word, ".,;:!?\"'()[]{}"))
		if len(word) >= 2 && !isStopWord(word) {
			wordCount[word]++
		}
	}

	// 按词频排序取 Top-K
	type wordFreq struct {
		word  string
		count int
	}
	var wfList []wordFreq
	for w, c := range wordCount {
		wfList = append(wfList, wordFreq{w, c})
	}

	// 简单冒泡排序（数据量小）
	for i := 0; i < len(wfList); i++ {
		for j := i + 1; j < len(wfList); j++ {
			if wfList[j].count > wfList[i].count {
				wfList[i], wfList[j] = wfList[j], wfList[i]
			}
		}
	}

	// 取 Top-K
	var keywords []string
	for i := 0; i < len(wfList) && i < maxKeywords; i++ {
		keywords = append(keywords, wfList[i].word)
	}

	return keywords
}

// isStopWord 检查是否是停用词
func isStopWord(word string) bool {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "is": true, "are": true, "was": true, "were": true,
		"be": true, "been": true, "being": true, "have": true, "has": true,
		"had": true, "do": true, "does": true, "did": true, "will": true,
		"would": true, "could": true, "should": true, "may": true, "might": true,
		"must": true, "shall": true, "can": true, "need": true, "dare": true,
		"to": true, "of": true, "in": true, "for": true, "on": true,
		"with": true, "at": true, "by": true, "from": true, "as": true,
		"into": true, "through": true, "during": true, "before": true, "after": true,
		"above": true, "below": true, "between": true, "under": true, "again": true,
		"further": true, "then": true, "once": true, "here": true, "there": true,
		"when": true, "where": true, "why": true, "how": true, "all": true,
		"each": true, "few": true, "more": true, "most": true, "other": true,
		"some": true, "such": true, "no": true, "nor": true, "not": true,
		"only": true, "own": true, "same": true, "so": true, "than": true,
		"too": true, "very": true, "just": true, "also": true, "now": true,
		"this": true, "that": true, "these": true, "those": true, "it": true,
		"its": true, "i": true, "me": true, "my": true, "we": true,
		"our": true, "you": true, "your": true, "he": true, "him": true,
		"his": true, "she": true, "her": true, "they": true, "them": true,
		"their": true, "what": true, "which": true, "who": true, "whom": true,
		// 中文停用词
		"的": true, "了": true, "是": true, "在": true, "有": true,
		"和": true, "与": true, "或": true, "等": true, "这": true,
		"那": true, "就": true, "也": true, "都": true, "要": true,
		"会": true, "能": true, "可以": true, "我": true, "你": true,
		"他": true, "她": true, "它": true, "们": true, "我们": true,
		"你们": true, "他们": true, "一个": true, "一些": true, "这个": true,
		"那个": true, "什么": true, "怎么": true, "为什么": true, "如何": true,
	}
	return stopWords[word]
}

// ParseFrontmatterOnly 仅解析 frontmatter（不读取完整 body）
func (p *FrontmatterParser) ParseFrontmatterOnly(content []byte) ([]byte, error) {
	fm, _, err := p.Parse(content)
	return fm, err
}
