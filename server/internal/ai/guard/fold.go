package guard

import "unicode/utf8"

// tradPairs 是"繁体字 简体字"成对的字符表，空格分隔，每一对恰好两个字符。
//
// 用途：红线检测的骨架视图里把繁体字形折成简体，挡住"换成繁体字写违规词"这类规避。
// 范围：只覆盖词表、句式规则与金融常用语里出现的字形，不追求完整的繁简转换
// （没有开源库可用，且完整转换不是这里的目标）。表里的键都是只有繁体写法的字，
// 不会把正常的简体文本改坏；新增词条时若涉及繁体字形，应同步补充。
//
// 成对写而不是写成 map 字面量，是为了避免在源码里写出相邻的词表词（合规门禁会扫描 .go 文件）。
const tradPairs = "龍龙 頭头 標标 純纯 偽伪 漲涨 穩稳 賺赚 搶抢 籌筹 潛潜 倉仓 價价 薦荐 買买 賣卖 強强 評评 將将 啟启 " +
	"發发 機机 會会 來来 輪轮 動动 議议 關关 級级 預预 計计 領领 確确 優优 選选 減减 風风 險险 無无 觀观 " +
	"點点 線线 區区 間间 壓压 撐撑 損损 虧亏 獲获 達达 過过 為为 於于 與与 對对 長长 開开 門门 場场 時时 " +
	"實实 現现 質质 業业 產产 經经 濟济 營营 務务 財财 貸贷 銀银 證证 資资 總总 華华 國国 後后 兩两 個个 " +
	"們们 這这 說说 話话 認认 覺觉 斷断 據据 歷历 麼么 裡里 擔担 當当 勢势 難难 順顺 還还 進进 運运 連连 " +
	"遠远 適适 應应 該该 讓让 從从 終终 並并 擁拥 護护 補补 規规 則则 範范 圍围 組组 織织 約约 結结 構构 " +
	"轉转 專专 項项 題题 師师 號号 檔档 顯显 調调 變变 異异 創创 億亿 萬万 幣币 額额 數数 續续 層层 競竞 " +
	"爭争 態态 況况 試试 驗验 屬属 類类 備备 設设 術术 網网 絡络 軟软 體体 電电 腦脑 車车 輛辆 藥药 醫医 " +
	"療疗 節节 環环 礦矿 鋰锂 鈷钴 鎳镍 鋁铝 鋼钢 鐵铁 銅铜 錫锡 鋅锌 鉛铅 導导 矽硅 積积 熱热 傳传 輸输 " +
	"歸归 戰战 艦舰 軍军 鐘钟 蘭兰 糧粮 農农 養养 豬猪 賬账 錢钱 " +
	"診诊 顧顾 問问 諮咨 詢询 稱称 報报 書书 劃划 負负 貨货 購购 賠赔 贏赢 盤盘 單单 擊击 衝冲 階阶 佈布 錯错"

// tradToSimp 由 tradPairs 在包初始化时解析。表本身有误（长度不对、重复键）会在初始化时直接 panic，
// 测试里 TestFoldTableWellFormed 也会覆盖；不让一张坏表悄悄进入生产。
var tradToSimp = buildFoldTable(tradPairs)

func buildFoldTable(pairs string) map[rune]rune {
	m := make(map[rune]rune, 256)
	start := 0
	flush := func(token string) {
		if token == "" {
			return
		}
		if utf8.RuneCountInString(token) != 2 {
			panic("guard: 繁简表条目必须恰好两个字符: " + token)
		}
		r := []rune(token)
		if _, dup := m[r[0]]; dup {
			panic("guard: 繁简表出现重复键: " + token)
		}
		m[r[0]] = r[1]
	}
	for i := 0; i < len(pairs); i++ {
		if pairs[i] == ' ' {
			flush(pairs[start:i])
			start = i + 1
		}
	}
	flush(pairs[start:])
	return m
}

// foldRune 把繁体字形折成简体；其它字符原样返回。
func foldRune(r rune) rune {
	if s, ok := tradToSimp[r]; ok {
		return s
	}
	return r
}
