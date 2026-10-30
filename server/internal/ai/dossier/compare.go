// Package dossier 提供专题资料的内部比较规则；不授予读取、发布或执行权限。
package dossier

import (
	"errors"
	"net/url"

	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

const (
	MaxBodyBytes = 1 << 20
	MaxSentences = 4096
)

var ErrInvalidComparison = errors.New("资料版本不符合比较条件")

// SentenceRef 只定位不可变快照，不携带正文；展示时仍须当前授权与引文回取。
type SentenceRef struct {
	SnapshotID int64
	Hash       string
	Number     int
	StartByte  int
	EndByte    int
}

// SnapshotDiff 是采集版本的文本差异，不是事实变化或撤回结论。
type SnapshotDiff struct {
	BeforeID, AfterID             int64
	BeforeHash, AfterHash         string
	BodyChanged                   bool
	CommonSentenceOrderChanged    bool
	Added, NotPresentInNewVersion []SentenceRef
}

// CompareSnapshots 比较同一文档的两个采集版本。调用者负责当前身份/许可；
// 本函数只核完整性与确定性差异，不调用模型、访问网络或修改账本。
func CompareSnapshots(before, after sources.Snapshot) (SnapshotDiff, error) {
	if !boundedSnapshot(before) || !boundedSnapshot(after) ||
		before.ID == after.ID || before.SourceID != after.SourceID ||
		before.SourceCategory != after.SourceCategory || before.URL != after.URL ||
		!before.FetchedAt.Before(after.FetchedAt) {
		return SnapshotDiff{}, ErrInvalidComparison
	}
	if before.Validate() != nil || after.Validate() != nil {
		return SnapshotDiff{}, ErrInvalidComparison
	}
	out := SnapshotDiff{
		BeforeID: before.ID, AfterID: after.ID,
		BeforeHash: before.Hash, AfterHash: after.Hash,
		BodyChanged: before.Hash != after.Hash,
		Added:       make([]SentenceRef, 0), NotPresentInNewVersion: make([]SentenceRef, 0),
	}
	// 同文句按出现次序配对，保留重复次数，不把句号偏移变化当作新增事实。
	positions := make(map[string][]int, len(after.Sentences))
	for i, sentence := range after.Sentences {
		positions[sentence.Text] = append(positions[sentence.Text], i)
	}
	matched := make([]bool, len(after.Sentences))
	last := -1
	for _, sentence := range before.Sentences {
		queue := positions[sentence.Text]
		if len(queue) == 0 {
			out.NotPresentInNewVersion = append(out.NotPresentInNewVersion, reference(before, sentence))
			continue
		}
		index := queue[0]
		positions[sentence.Text] = queue[1:]
		matched[index] = true
		if index < last {
			out.CommonSentenceOrderChanged = true
		}
		last = index
	}
	for i, sentence := range after.Sentences {
		if !matched[i] {
			out.Added = append(out.Added, reference(after, sentence))
		}
	}
	return out, nil
}

func boundedSnapshot(s sources.Snapshot) bool {
	if s.ID <= 0 || len(s.Body) == 0 || len(s.Body) > MaxBodyBytes ||
		len(s.Sentences) == 0 || len(s.Sentences) > MaxSentences || s.FetchedAt.UnixMilli() <= 0 {
		return false
	}
	u, err := url.Parse(s.URL)
	if err != nil {
		return false
	}
	canonical, err := sources.ValidateURL(u.Hostname(), s.URL)
	return err == nil && canonical == s.URL
}

func reference(s sources.Snapshot, sentence sources.Sentence) SentenceRef {
	return SentenceRef{SnapshotID: s.ID, Hash: s.Hash, Number: sentence.Number,
		StartByte: sentence.StartByte, EndByte: sentence.EndByte}
}
