// Package v2 提供记忆自动识别与分类功能
package v2

import (
	"regexp"
	"strings"
)

// MemoryClassifier 记忆分类器
// 负责从用户输入中识别和分类记忆
type MemoryClassifier struct {
	// 偏好识别模式
	preferencePatterns []*regexp.Regexp

	// 规则识别模式
	rulePatterns []*regexp.Regexp

	// 临时信息识别模式
	temporaryPatterns []*regexp.Regexp

	// 项目相关识别模式
	projectPatterns []*regexp.Regexp
}

// ClassificationResult 分类结果
type ClassificationResult struct {
	// 是否应该存储为记忆
	ShouldStore bool `json:"should_store"`

	// 推荐的记忆类型
	MemoryType MemoryType `json:"memory_type"`

	// 推荐的分类
	Category MemoryCategory `json:"category"`

	// 推荐的作用域
	Scope MemoryScope `json:"scope"`

	// 提取的标题
	Title string `json:"title"`

	// 置信度 (0-1)
	Confidence float64 `json:"confidence"`

	// 推荐的 TTL（天，仅短期记忆）
	TTLDays int `json:"ttl_days,omitempty"`

	// 提取的标签
	Tags []string `json:"tags,omitempty"`

	// 分类原因
	Reason string `json:"reason"`
}

// NewMemoryClassifier 创建记忆分类器
func NewMemoryClassifier() *MemoryClassifier {
	c := &MemoryClassifier{}
	c.initPatterns()
	return c
}

// initPatterns 初始化识别模式
func (c *MemoryClassifier) initPatterns() {
	// 用户偏好模式
	c.preferencePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)我(习惯|喜欢|偏好|倾向于?)`),
		regexp.MustCompile(`(?i)我(一般|通常|总是|经常)`),
		regexp.MustCompile(`(?i)我(希望你|想让你|要求你)`),
		regexp.MustCompile(`(?i)(记住|记下|记得).*(偏好|习惯|喜欢)`),
		regexp.MustCompile(`(?i)我的(风格|方式|习惯)是`),
		regexp.MustCompile(`(?i)从现在开始.*(一直|总是|始终)`),
		regexp.MustCompile(`(?i)以后.*(都|总是|一直)`),
		regexp.MustCompile(`(?i)I (prefer|like|want|always|usually)`),
		regexp.MustCompile(`(?i)please (remember|note|keep in mind)`),
	}

	// 规则模式
	c.rulePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(不要|禁止|不允许|别)`),
		regexp.MustCompile(`(?i)(必须|一定要|务必|强制)`),
		regexp.MustCompile(`(?i)(规则|约定|规范|标准)是`),
		regexp.MustCompile(`(?i)(永远|始终|任何时候)(不要|都要)`),
		regexp.MustCompile(`(?i)don't|never|always|must`),
		regexp.MustCompile(`(?i)以后不要`),
		regexp.MustCompile(`(?i)从不`),
	}

	// 临时信息模式
	c.temporaryPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(今天|明天|今晚|今早)`),
		regexp.MustCompile(`(?i)(这周|本周|下周|这个月|本月)`),
		regexp.MustCompile(`(?i)(马上|立刻|待会|一会儿)`),
		regexp.MustCompile(`(?i)(临时|暂时|先)`),
		regexp.MustCompile(`(?i)today|tomorrow|this week|next week`),
		regexp.MustCompile(`(?i)\d{1,2}(点|时|分)`),
		regexp.MustCompile(`(?i)\d{1,2}(月|号|日)`),
	}

	// 项目相关模式
	c.projectPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(这个|当前|本)(项目|工程|代码库|仓库)`),
		regexp.MustCompile(`(?i)(项目|工程)的?(架构|结构|设计|技术栈)`),
		regexp.MustCompile(`(?i)(代码|文件|模块|组件)的?(位置|路径|结构)`),
		regexp.MustCompile(`(?i)this (project|codebase|repository)`),
	}
}

// Classify 对输入文本进行分类
func (c *MemoryClassifier) Classify(text string) *ClassificationResult {
	result := &ClassificationResult{
		ShouldStore: false,
		Confidence:  0,
	}

	// 文本过短，可能不是记忆
	if len(text) < 10 {
		result.Reason = "文本过短"
		return result
	}

	// 检测用户偏好
	if c.matchAny(text, c.preferencePatterns) {
		result.ShouldStore = true
		result.MemoryType = MemoryTypeCore
		result.Category = CategoryPreference
		result.Scope = ScopeGlobal
		result.Confidence = 0.8
		result.Title = c.extractTitle(text, "用户偏好")
		result.Reason = "检测到用户偏好表达"
		return result
	}

	// 检测规则
	if c.matchAny(text, c.rulePatterns) {
		result.ShouldStore = true
		result.MemoryType = MemoryTypeCore
		result.Category = CategoryRule
		result.Scope = ScopeGlobal
		result.Confidence = 0.85
		result.Title = c.extractTitle(text, "用户规则")
		result.Reason = "检测到用户规则表达"
		return result
	}

	// 检测临时信息
	if c.matchAny(text, c.temporaryPatterns) {
		result.ShouldStore = true
		result.MemoryType = MemoryTypeShortTerm

		// 根据时间词决定分类和 TTL
		if c.containsTaskKeywords(text) {
			result.Category = CategoryTask
			result.TTLDays = 3
		} else {
			result.Category = CategoryNote
			result.TTLDays = 7
		}

		// 检测是否项目相关
		if c.matchAny(text, c.projectPatterns) {
			result.Scope = ScopeProject
		} else {
			result.Scope = ScopeGlobal
		}

		result.Confidence = 0.7
		result.Title = c.extractTitle(text, "临时笔记")
		result.Reason = "检测到临时性时间表达"
		return result
	}

	// 检测项目知识
	if c.matchAny(text, c.projectPatterns) && c.isKnowledgeContent(text) {
		result.ShouldStore = true
		result.MemoryType = MemoryTypeLongTerm
		result.Category = CategoryProject
		result.Scope = ScopeProject
		result.Confidence = 0.75
		result.Title = c.extractTitle(text, "项目知识")
		result.Tags = c.extractTags(text)
		result.Reason = "检测到项目相关知识"
		return result
	}

	// 检测通用知识
	if c.isKnowledgeContent(text) {
		result.ShouldStore = true
		result.MemoryType = MemoryTypeLongTerm
		result.Category = CategoryKnowledge
		result.Scope = ScopeGlobal
		result.Confidence = 0.6
		result.Title = c.extractTitle(text, "知识记录")
		result.Tags = c.extractTags(text)
		result.Reason = "检测到知识性内容"
		return result
	}

	result.Reason = "未检测到需要存储的记忆特征"
	return result
}

// ClassifyFromConversation 从对话中识别记忆
func (c *MemoryClassifier) ClassifyFromConversation(userMessage, assistantResponse string) *ClassificationResult {
	// 首先分析用户消息
	result := c.Classify(userMessage)

	// 如果用户消息识别为记忆，直接返回
	if result.ShouldStore {
		return result
	}

	// 分析是否是显式记忆指令
	if c.isExplicitMemoryCommand(userMessage) {
		result.ShouldStore = true
		result.MemoryType = MemoryTypeCore
		result.Category = CategoryPreference
		result.Scope = ScopeGlobal
		result.Confidence = 0.9
		result.Title = c.extractTitle(userMessage, "用户指令")
		result.Reason = "检测到显式记忆指令"
		return result
	}

	return result
}

// matchAny 检查文本是否匹配任一模式
func (c *MemoryClassifier) matchAny(text string, patterns []*regexp.Regexp) bool {
	for _, p := range patterns {
		if p.MatchString(text) {
			return true
		}
	}
	return false
}

// containsTaskKeywords 检查是否包含任务相关关键词
func (c *MemoryClassifier) containsTaskKeywords(text string) bool {
	keywords := []string{
		"任务", "待办", "要做", "完成", "实现", "修复", "添加",
		"task", "todo", "fix", "implement", "add", "create",
	}

	textLower := strings.ToLower(text)
	for _, kw := range keywords {
		if strings.Contains(textLower, kw) {
			return true
		}
	}
	return false
}

// isKnowledgeContent 判断是否是知识性内容
func (c *MemoryClassifier) isKnowledgeContent(text string) bool {
	// 知识性内容特征
	knowledgePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(是|为|叫做|称为|表示|意味着)`),
		regexp.MustCompile(`(?i)(可以|能够|用于|用来)`),
		regexp.MustCompile(`(?i)(包含|包括|由.*组成)`),
		regexp.MustCompile(`(?i)(定义|概念|原理|方法)`),
		regexp.MustCompile(`(?i)(技术|框架|工具|库|API)`),
		regexp.MustCompile(`(?i)is|are|means|represents`),
		regexp.MustCompile(`(?i)can be|used for|consists of`),
	}

	matchCount := 0
	for _, p := range knowledgePatterns {
		if p.MatchString(text) {
			matchCount++
		}
	}

	// 需要匹配至少2个知识性模式
	return matchCount >= 2
}

// isExplicitMemoryCommand 判断是否是显式记忆指令
func (c *MemoryClassifier) isExplicitMemoryCommand(text string) bool {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)^(记住|记下|记得|保存|存储)`),
		regexp.MustCompile(`(?i)(请|帮我)?(记住|记下|保存)`),
		regexp.MustCompile(`(?i)^(remember|save|store|note)`),
		regexp.MustCompile(`(?i)please (remember|save|note)`),
	}

	for _, p := range patterns {
		if p.MatchString(text) {
			return true
		}
	}
	return false
}

// extractTitle 从文本中提取标题
func (c *MemoryClassifier) extractTitle(text, defaultTitle string) string {
	// 尝试提取第一句话作为标题
	sentences := regexp.MustCompile(`[.。!！?？\n]`).Split(text, 2)
	if len(sentences) > 0 && len(sentences[0]) > 0 {
		title := strings.TrimSpace(sentences[0])
		// 限制长度
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		if len(title) >= 5 {
			return title
		}
	}

	return defaultTitle
}

// extractTags 从文本中提取标签
func (c *MemoryClassifier) extractTags(text string) []string {
	var tags []string

	// 技术相关标签
	techPatterns := map[string]*regexp.Regexp{
		"go":         regexp.MustCompile(`(?i)\b(golang|go语言|\.go)\b`),
		"python":     regexp.MustCompile(`(?i)\b(python|\.py)\b`),
		"javascript": regexp.MustCompile(`(?i)\b(javascript|js|nodejs|node\.js)\b`),
		"typescript": regexp.MustCompile(`(?i)\b(typescript|ts)\b`),
		"react":      regexp.MustCompile(`(?i)\b(react|reactjs)\b`),
		"vue":        regexp.MustCompile(`(?i)\b(vue|vuejs)\b`),
		"database":   regexp.MustCompile(`(?i)\b(sql|数据库|database|mysql|postgres|mongodb)\b`),
		"api":        regexp.MustCompile(`(?i)\b(api|接口|rest|graphql)\b`),
		"docker":     regexp.MustCompile(`(?i)\b(docker|容器|kubernetes|k8s)\b`),
		"git":        regexp.MustCompile(`(?i)\b(git|github|gitlab)\b`),
	}

	for tag, pattern := range techPatterns {
		if pattern.MatchString(text) {
			tags = append(tags, tag)
		}
	}

	// 限制标签数量
	if len(tags) > 5 {
		tags = tags[:5]
	}

	return tags
}

// SuggestMemoryAction 建议记忆操作
func (c *MemoryClassifier) SuggestMemoryAction(text string) *MemoryAction {
	result := c.Classify(text)

	action := &MemoryAction{
		Action:   "none",
		Reason:   result.Reason,
		Metadata: result,
	}

	if !result.ShouldStore {
		return action
	}

	switch result.MemoryType {
	case MemoryTypeCore:
		action.Action = "store_core"
	case MemoryTypeShortTerm:
		action.Action = "store_short_term"
	case MemoryTypeLongTerm:
		action.Action = "store_long_term"
	}

	return action
}

// MemoryAction 记忆操作建议
type MemoryAction struct {
	Action   string                `json:"action"` // store_core, store_short_term, store_long_term, none
	Reason   string                `json:"reason"`
	Metadata *ClassificationResult `json:"metadata,omitempty"`
}

// DetermineScope 根据上下文确定记忆作用域
func (c *MemoryClassifier) DetermineScope(text string, hasProject bool) MemoryScope {
	// 如果没有项目上下文，默认全局
	if !hasProject {
		return ScopeGlobal
	}

	// 检查是否明确提到项目
	if c.matchAny(text, c.projectPatterns) {
		return ScopeProject
	}

	// 检查是否明确提到全局
	globalPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(全局|所有项目|任何项目|通用)`),
		regexp.MustCompile(`(?i)(global|all projects|any project|universal)`),
	}
	if c.matchAny(text, globalPatterns) {
		return ScopeGlobal
	}

	// 默认跟随当前项目
	return ScopeProject
}

// ExtractImportance 从文本中提取重要性
func (c *MemoryClassifier) ExtractImportance(text string) int {
	// 高重要性关键词
	highPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(非常|特别|极其|绝对)(重要|关键)`),
		regexp.MustCompile(`(?i)(必须|一定|务必)`),
		regexp.MustCompile(`(?i)(critical|crucial|essential|must)`),
	}

	// 低重要性关键词
	lowPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(可能|也许|或许)`),
		regexp.MustCompile(`(?i)(minor|optional|nice to have)`),
	}

	if c.matchAny(text, highPatterns) {
		return 5
	}
	if c.matchAny(text, lowPatterns) {
		return 2
	}

	return 3 // 默认中等重要性
}
