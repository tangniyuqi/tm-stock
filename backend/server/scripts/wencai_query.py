import sys
import json
import warnings
import argparse

# 忽略所有警告，防止污染 stdout 导致 JSON 解析失败
warnings.filterwarnings("ignore")

import pywencai

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('query', type=str, help='The search query')
    parser.add_argument('--pro', action='store_true', help='Use pro version')
    parser.add_argument('--cookie', type=str, default=None, help='Cookie for pro version')

    args, unknown = parser.parse_known_args()
    
    query = args.query
    pro = args.pro
    cookie = args.cookie

    try:
        kwargs = {'query': query, 'pro': pro, 'cookie': cookie, 'loop': False}
        res = pywencai.get(**kwargs)
        if res is None:
            print(json.dumps({"error": "No result returned"}))
            return
            
        import pandas as pd
        if isinstance(res, pd.DataFrame):
            # Convert to dict records
            data = res.to_dict(orient='records')
            print(json.dumps({"data": data}, ensure_ascii=False))
        elif isinstance(res, dict):
            print(json.dumps({"data": res}, ensure_ascii=False))
        else:
            print(json.dumps({"data": str(res)}, ensure_ascii=False))
    except Exception as e:
        print(json.dumps({"error": str(e)}, ensure_ascii=False))

if __name__ == '__main__':
    main()
