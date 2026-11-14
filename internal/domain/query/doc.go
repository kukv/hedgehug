// Package query は、検索のクエリ(`status:done -assignee:@me sort:-priority` のような文字列)を語に切り、
// キーと値とエラーを決める。一致と並びの判定は、この解析の結果の上に置く。
package query
