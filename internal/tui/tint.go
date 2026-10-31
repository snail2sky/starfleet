package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/x/ansi"
)

// 表格着色说明（为什么不在单元格里直接放颜色）：
//
// bubbles v1.0.0 的表格用 runewidth.Truncate 计算宽度——它**不识别 ANSI**，
// 会把颜色转义序列的字符当成可见字符，导致：截断错位、列与列不再对齐、
// 转义被拦腰截断。而且选中行整体包了 Selected 样式，单元格内的 reset
// 会把行背景打断（光标划过时一行行“闪色”）。
//
// 因此：单元格保持纯文本（对齐永远精确），渲染完成后只给**非选中行**的
// 特定列补色；选中行保持整行统一高亮。
// tintCell 给某个纯文本单元格片段套上语义色。
func tintCell(chunk string) string {
	switch strings.TrimSpace(ansi.Strip(chunk)) {
	case "on", "always", "RUNNING":
		return SuccessStyle.Render(chunk)
	case "off":
		return ErrorStyle.Render(chunk)
	case "unexpected", "BACKOFF", "STARTING", "STOPPING":
		return WarningStyle.Render(chunk)
	case "never", "STOPPED", "EXITED":
		return MutedStyle.Render(chunk)
	case "FATAL":
		return ErrorStyle.Render(chunk)
	}
	return chunk
}

// openSGR 提取一样式渲染空串时的“开码”前缀，用于识别选中行。
func openSGR(s string) string {
	// 形如 \x1b[...m ；取第一个终止符之前的全部
	if !strings.HasPrefix(s, "\x1b") {
		return ""
	}
	if i := strings.IndexByte(s, 'm'); i >= 0 {
		return s[:i+1]
	}
	return ""
}

// colorizeTableView 为渲染后的表格补色：
//   - 表头/空行不动
//   - 含 Selected 开码的行（光标所在行）不动 → 整行统一高亮
//   - 其余行按列切分（列宽+2 的单元格内边距，ansi 感知），值为
//     on/off/always/... 或进程状态的单元格上色
func colorizeTableView(view string, cols []table.Column, styles table.Styles) string {
	lines := strings.Split(view, "\n")
	if len(lines) < 2 {
		return view
	}
	sel := openSGR(styles.Selected.Render(""))
	for i, ln := range lines {
		if i == 0 || strings.TrimSpace(ln) == "" {
			continue // 表头与分隔空行
		}
		if sel != "" && strings.Contains(ln, sel) {
			continue // 选中行保持统一
		}
		lines[i] = tintColumns(ln, cols)
	}
	return strings.Join(lines, "\n")
}

// tintColumns 按列宽切分一行（每列含左右各1的内边距），对命中词上色。
func tintColumns(line string, cols []table.Column) string {
	var b strings.Builder
	rest := line
	for _, c := range cols {
		if c.Width <= 0 {
			continue
		}
		w := c.Width + 2 // bubbles 的 Cell 内边距：左右各1
		chunk := ansi.Truncate(rest, w, "")
		rest = ansi.TruncateLeft(rest, w, "")
		b.WriteString(tintCell(chunk))
	}
	b.WriteString(rest)
	return b.String()
}
