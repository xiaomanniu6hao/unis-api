#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
从 new-api 日志库导出请求内容到 JSON 文件。

默认导出"今天"（本地时区 Asia/Shanghai）的消费日志（type=2，记录用户请求的日志）。
每条记录包含：
  - request_id            new-api 内部请求 ID
  - upstream_request_id   上游返回的请求 ID
  - user_input            用户输入内容（仅 Claude 且 LogUserInputEnabled 开启时记录，否则为空）
  - content               日志描述文本
  - 以及 id / created_at / type / username / model_name / token_id / token_name 便于溯源

用法:
  python export_logs.py                      # 导出今天的消费日志
  python export_logs.py --date 2026-07-20    # 导出指定日期（本地时区当天 00:00 ~ 次日 00:00）
  python export_logs.py --start 1719504000 --end 1719583999  # 用 unix 时间戳精确指定
  python export_logs.py --out /path/to/logs.json

依赖: pip install pymysql
"""

import argparse
import json
import os
import sys
from datetime import datetime, timedelta, timezone, tzinfo

try:
    import pymysql
except ImportError:
    sys.stderr.write("缺少依赖 pymysql，请先安装: pip install pymysql\n")
    sys.exit(1)


# Asia/Shanghai 时区（不依赖 zoneinfo，避免 Windows 下缺数据包）
class ShanghaiTz(tzinfo):
    _OFFSET = timezone(timedelta(hours=8))

    def utcoffset(self, dt):
        return self._OFFSET.utcoffset(dt)

    def tzname(self, dt):
        return "Asia/Shanghai"

    def dst(self, dt):
        return timedelta(0)


TZ = ShanghaiTz()

# 默认数据库连接（与 docker-compose.yml 中 SQL_DSN 一致）
DEFAULT_DB_HOST = os.environ.get("UNISAPI_DB_HOST", "127.0.0.1")
DEFAULT_DB_PORT = int(os.environ.get("UNISAPI_DB_PORT", "3307"))
DEFAULT_DB_USER = os.environ.get("UNISAPI_DB_USER", "unisapi")
DEFAULT_DB_PASS = os.environ.get("UNISAPI_DB_PASS", "123456")
DEFAULT_DB_NAME = os.environ.get("UNISAPI_DB_NAME", "unisapi")

LOG_TYPE_NAMES = {
    0: "unknown",
    1: "topup",
    2: "consume",
    3: "manage",
    4: "system",
    5: "error",
    6: "refund",
    7: "login",
}

# 默认只导出消费日志（type=2），这才是记录用户请求的日志；
# error/topup/manage 等其它类型不含用户请求内容。
DEFAULT_LOG_TYPE = LogTypeConsume = 2


def parse_date_to_range(date_str):
    """把 YYYY-MM-DD 解析成本地时区 [当天00:00, 次日00:00) 的 unix 时间戳区间。"""
    day = datetime.strptime(date_str, "%Y-%m-%d").replace(tzinfo=TZ)
    start = int(day.timestamp())
    end = int((day + timedelta(days=1)).timestamp())
    return start, end


def today_range():
    today = datetime.now(TZ).date()
    return parse_date_to_range(today.isoformat())


def fetch_logs(host, port, user, passwd, db, start_ts, end_ts):
    conn = pymysql.connect(
        host=host,
        port=port,
        user=user,
        password=passwd,
        database=db,
        charset="utf8mb4",
        cursorclass=pymysql.cursors.DictCursor,
    )
    try:
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT id, request_id, upstream_request_id, user_input, content,
                       created_at, type, username, model_name, token_id, token_name
                FROM logs
                WHERE created_at >= %s AND created_at < %s AND type = %s
                ORDER BY created_at ASC, id ASC
                """,
                (start_ts, end_ts, DEFAULT_LOG_TYPE),
            )
            rows = cur.fetchall()
    finally:
        conn.close()
    return rows


def row_to_obj(row):
    created_at = row.get("created_at")
    return {
        "id": row.get("id"),
        "request_id": row.get("request_id") or "",
        "upstream_request_id": row.get("upstream_request_id") or "",
        "user_input": row.get("user_input") or "",
        "content": row.get("content") or "",
        "created_at": created_at,
        "created_at_iso": (
            datetime.fromtimestamp(created_at, TZ).isoformat()
            if created_at is not None
            else None
        ),
        "type": row.get("type"),
        "type_name": LOG_TYPE_NAMES.get(row.get("type"), "unknown"),
        "username": row.get("username") or "",
        "model_name": row.get("model_name") or "",
        "token_id": row.get("token_id") or 0,
        "token_name": row.get("token_name") or "",
    }


def main():
    ap = argparse.ArgumentParser(description="从 new-api 日志库导出请求内容到 JSON")
    ap.add_argument("--date", help="导出指定日期 YYYY-MM-DD（本地时区，默认今天）")
    ap.add_argument("--start", type=int, help="起始 unix 时间戳（与 --end 同时使用，优先于 --date）")
    ap.add_argument("--end", type=int, help="结束 unix 时间戳（左闭右开）")
    ap.add_argument("--out", help="输出 JSON 路径（默认 ./logs_export_<date>.json）")
    ap.add_argument("--host", default=DEFAULT_DB_HOST)
    ap.add_argument("--port", type=int, default=DEFAULT_DB_PORT)
    ap.add_argument("--user", default=DEFAULT_DB_USER)
    ap.add_argument("--password", default=DEFAULT_DB_PASS)
    ap.add_argument("--db", default=DEFAULT_DB_NAME)
    args = ap.parse_args()

    if args.start is not None and args.end is not None:
        start_ts, end_ts = args.start, args.end
        date_label = f"{start_ts}_{end_ts}"
    else:
        if args.date:
            start_ts, end_ts = parse_date_to_range(args.date)
            date_label = args.date
        else:
            start_ts, end_ts = today_range()
            date_label = datetime.now(TZ).date().isoformat()

    out_path = args.out or f"logs_export_{date_label}.json"

    start_iso = datetime.fromtimestamp(start_ts, TZ).isoformat()
    end_iso = datetime.fromtimestamp(end_ts, TZ).isoformat()
    sys.stderr.write(
        f"导出区间 {start_iso} ~ {end_iso}（左闭右开），连接 {args.host}:{args.port}/{args.db}\n"
    )

    rows = fetch_logs(args.host, args.port, args.user, args.password, args.db, start_ts, end_ts)
    records = [row_to_obj(r) for r in rows]

    with open(out_path, "w", encoding="utf-8") as f:
        json.dump(
            {
                "range": {"start": start_ts, "end": end_ts},
                "type": DEFAULT_LOG_TYPE,
                "type_name": LOG_TYPE_NAMES[DEFAULT_LOG_TYPE],
                "count": len(records),
                "records": records,
            },
            f,
            ensure_ascii=False,
            indent=2,
        )

    sys.stderr.write(f"已导出 {len(records)} 条到 {out_path}\n")


if __name__ == "__main__":
    main()
