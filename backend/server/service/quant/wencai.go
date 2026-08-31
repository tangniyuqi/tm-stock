package quant

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
)

type WencaiService struct{}

func (s *WencaiService) Query(query string, pro bool, cookie string) (interface{}, error) {
	return s.QueryWithContext(context.Background(), query, pro, cookie)
}

func (s *WencaiService) QueryWithContext(ctx context.Context, query string, pro bool, cookie string) (interface{}, error) {
	args := []string{"scripts/wencai_query.py", query}
	if pro {
		args = append(args, "--pro")
		if cookie != "" {
			args = append(args, "--cookie", cookie)
		}
	}

	cmd := exec.CommandContext(ctx, "python3", args...)
	out, err := cmd.Output()
	if err != nil {
		// 检查是否是上下文取消
		if ctx.Err() != nil {
			return nil, errors.New("查询已取消")
		}
		
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return nil, errors.New("脚本执行失败: " + string(exitError.Stderr))
		}
		return nil, errors.New("脚本执行失败: " + err.Error())
	}

	// 尝试寻找第一个 '{'，防止输出中包含无关的开头信息
	outStr := string(out)
	firstBrace := -1
	for i, c := range outStr {
		if c == '{' {
			firstBrace = i
			break
		}
	}
	if firstBrace == -1 {
		return nil, errors.New("未找到有效的JSON输出: " + outStr)
	}
	outStr = outStr[firstBrace:]

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(outStr), &result); err != nil {
		return nil, errors.New("解析JSON失败: " + err.Error() + "\n原始输出片段: " + outStr)
	}

	// if errMsg, ok := result["error"]; ok {
	// 	return nil, errors.New("问财查询报错: " + errMsg.(string))
	// }

	return result["data"], nil
}
