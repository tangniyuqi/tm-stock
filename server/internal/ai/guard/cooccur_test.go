package guard

import "testing"

func TestCoOccurTable(t *testing.T) {
	names := []string{"星河科技", "300001"}
	anchors := []string{"算力", "智算"}
	cases := []struct {
		name    string
		quote   string
		names   []string
		anchors []string
		want    CoReason
	}{
		{"同句共现（简称）", "星河科技是国内领先的算力服务商。", names, anchors, CoOK},
		{"同句共现（证券代码）", "证券代码300001的公司主营智算中心运营。", names, anchors, CoOK},
		{"别名里的空格被忽略", "星河 科技主营算 力服务。", names, anchors, CoOK},
		{"大小写不敏感", "TCL主营AI算力服务。", []string{"tcl"}, []string{"ai"}, CoOK},
		{"没有别名", "云岫算力主营算力服务。", names, anchors, CoNoEntity},
		{"没有锚词", "星河科技主营光伏组件。", names, anchors, CoNoAnchor},
		{"实体与锚词在不同句子", "星河科技成立于2010年。公司主营算力服务。", names, anchors, CoNotSameSentence},
		{"被换行隔开也算不同句", "星河科技成立于2010年\n公司主营算力服务", names, anchors, CoNotSameSentence},
		{"别名为空", "星河科技主营算力。", nil, anchors, CoUnusableAlias},
		{"锚词为空", "星河科技主营算力。", names, nil, CoUnusableAlias},
		{"别名短于两个字符被忽略后为空", "星河科技主营算力。", []string{"星", ""}, anchors, CoUnusableAlias},
		{"锚词短于两个字符被忽略后为空", "星河科技主营算力。", names, []string{"算"}, CoUnusableAlias},
		{"代码被更长的数字吞掉不算命中", "编号1300001的公司主营算力服务。", []string{"300001"}, anchors, CoNoEntity},
		{"代码前后紧邻字母也不算命中", "编号A300001B的公司主营算力服务。", []string{"300001"}, anchors, CoNoEntity},
		{"ASCII 锚词被更长字母吞掉不算命中", "星河科技主营MAIN业务。", names, []string{"ai"}, CoNoAnchor},
		{"否定提示：不涉及", "星河科技不涉及算力业务。", names, anchors, CoNegatedAnchor},
		{"否定提示：并非", "星河科技并非算力企业。", names, anchors, CoNegatedAnchor},
		{"否定提示：未", "星河科技未开展算力业务。", names, anchors, CoNegatedAnchor},
		{"否定提示：无（间隔一个字）", "星河科技无涉算力。", names, anchors, CoNegatedAnchor},
		{"否定提示间隔三个字不算（边界：最多两个）", "星河科技无意涉足算力。", names, anchors, CoOK},
		{"否定提示间隔超过两个字不算", "星河科技并非一直从事算力业务。", names, anchors, CoOK},
		{"不仅不是否定提示", "星河科技不仅提供算力服务，还提供运维。", names, anchors, CoOK},
		{"锚词自身含否定字不算否定", "星河科技主营无人机算力调度。", names, []string{"无人机"}, CoOK},
		{"同句有一处否定一处肯定则通过", "星河科技并非智算企业，但主营算力服务。", names, anchors, CoOK},

		// 别名里自带锚词：提到公司名不等于披露了该业务
		{"别名含锚词：只提公司名不算共现", "云岫算力召开了年度股东大会。", []string{"云岫算力"}, anchors, CoNoAnchor},
		{"别名含锚词：公司名之外另有独立的锚词才算", "云岫算力提供算力租赁服务。", []string{"云岫算力"}, anchors, CoOK},
		{"别名含锚词：锚词与别名部分重叠也不算", "星河科技概念受到关注。", names, []string{"科技概念"}, CoNoAnchor},
		{"别名含锚词：不同句里的独立锚词不算同句", "云岫算力召开了股东大会。公司主营算力服务。", []string{"云岫算力"}, anchors, CoNotSameSentence},
		{"别名含锚词：独立锚词前有否定仍判否定", "云岫算力并未开展算力租赁业务。", []string{"云岫算力"}, anchors, CoNegatedAnchor},

		// 别名里自带的否定字不是否定词
		{"别名里的无不是否定", "无锡算力公司提供服务。", []string{"无锡"}, anchors, CoOK},
		{"别名里的非不是否定", "非凡科技主营算力服务。", []string{"非凡科技"}, anchors, CoOK},
		{"别名之外的否定词照常生效", "无锡公司并非算力企业。", []string{"无锡"}, anchors, CoNegatedAnchor},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CoOccur(c.quote, c.names, c.anchors); got != c.want {
				t.Fatalf("CoOccur(%q) = %q，期望 %q", c.quote, got, c.want)
			}
		})
	}
}

func TestUsableTermsDedupesAndFilters(t *testing.T) {
	got := usableTerms([]string{"星河科技", " 星河 科技 ", "A", "", "ＡＩ", "ai"})
	if len(got) != 2 || string(got[0]) != "星河科技" || string(got[1]) != "ai" {
		t.Fatalf("usableTerms = %q", got)
	}
}

func TestContainsCompact(t *testing.T) {
	for _, c := range []struct {
		hay, needle string
		want        bool
	}{
		{"星河 科技主营算力", "星河科技", true},
		{"星河……科技主营算力", "星河科技", false},
		{"主营算力", "", false},
		{"ABC公司", "abc", true},
	} {
		if got := containsCompact(c.hay, c.needle); got != c.want {
			t.Errorf("containsCompact(%q,%q) = %v，期望 %v", c.hay, c.needle, got, c.want)
		}
	}
}
