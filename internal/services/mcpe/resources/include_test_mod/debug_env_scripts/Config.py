# -*- coding: utf-8 -*-
import os
from json import loads

_DEBUG_INFO = loads(os.environ.get("MCS_HELPER_DEBUG_OPTIONS", "{}"))
_TARGET_MOD_DIRS = loads(os.environ.get("MCS_HELPER_TARGET_MOD_DIRS", "[]"))

try:
    DEBUG_CONFIG = loads(_DEBUG_INFO) if not isinstance(_DEBUG_INFO, dict) else _DEBUG_INFO
except:
    DEBUG_CONFIG = {}

try:
    TARGET_MOD_DIRS = loads(_TARGET_MOD_DIRS) if not isinstance(_TARGET_MOD_DIRS, list) else _TARGET_MOD_DIRS
except:
    TARGET_MOD_DIRS = []


def GET_DEBUG_IPC_PORT():
    port = os.getenv("MCDEV_DEBUG_IPC_PORT")
    if port is None:
        return None
    return int(port)


def GET_DEBUG_RUN_ID():
    # run 归属标识：XK-Pane 每次启动注入，桥模组连上后用 hello 帧回传，
    # 让多开客户端的日志在 XK-Pane 侧按 run 精确归属。
    return os.getenv("MCDEV_DEBUG_RUN_ID") or ""
