# -*- coding: utf-8 -*-
import socket
import threading
import mod.client.extraClientApi as clientApi
from .Config import GET_DEBUG_IPC_PORT

def U16_BE(b):
    # type: (bytearray | str) -> int
    if isinstance(b, bytearray):
        return (b[0] << 8) | b[1]
    return (ord(b[0]) << 8) | ord(b[1])

def U32_BE(b):
    # type: (bytearray | str) -> int
    if isinstance(b, bytearray):
        return (b[0] << 24) | (b[1] << 16) | (b[2] << 8) | b[3]
    return (ord(b[0]) << 24) | (ord(b[1]) << 16) | (ord(b[2]) << 8) | ord(b[3])

class IPCSystem:
    def __init__(self, port=None):
        # type: (int | None) -> None
        self.port = port
        self.sock = None
        self.mLock = threading.Lock()
        self.handers = {}

    def registerHandler(self, typeID, handler):
        # type: (int, callable) -> None
        self.handers[typeID] = handler

    def updateHandlers(self, handlers):
        # type: (dict[int, callable]) -> None
        self.handers.update(handlers)

    def start(self):
        if self.sock or not self.port:
            return
        threading.Thread(target=self._threadListenLoop).start()

    def close(self):
        sock = None
        with self.mLock:
            sock = self.sock
            self.sock = None
        if sock:
            sock.close()

    def _threadListenLoop(self):
        with self.mLock:
            self.sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock = self.sock
        sock.connect(("localhost", self.port))
        # [2B TypeID][4B DataLength][Data]
        def _recvAll(sock, length):
            # type: (socket.socket, int) -> bytearray
            buf = bytearray()
            while len(buf) < length:
                more = sock.recv(length - len(buf))
                if not more:
                    raise EOFError("Socket closed before receiving all data")
                buf.extend(more)
            return buf
        while 1:
            try:
                header = _recvAll(sock, 6)
            except EOFError:
                break
            except Exception:
                import traceback
                traceback.print_exc()
                break
            typeID = U16_BE(header[0:2])
            dataLength = U32_BE(header[2:6])
            data = _recvAll(sock, dataLength)
            if typeID in self.handers:
                try:
                    self.handers[typeID](data)
                except Exception:
                    import traceback
                    traceback.print_exc()
            else:
                print("[IPCSystem] 未知的TypeID数据包：", typeID)
        with self.mLock:
            self.sock = None

GAME_COMP = None

def _ON_MAIN_THREAD(fn):
    # type: (callable) -> None
    # IPC 接收线程 -> 客户端主线程：ModSDK 的 reload / exec 都得在主线程跑，
    # 否则 RELOAD_WORLD / refresh_addons 这种 API 会抛 thread-affinity 错。
    # GAME_COMP 还没就绪就直接调（启动早期），让 mod 自己抛错更直观。
    if GAME_COMP:
        GAME_COMP.AddTimer(0, fn)
    else:
        fn()

def AUTO_RELOAD(_=None):
    from .Game import RELOAD_MOD
    _ON_MAIN_THREAD(lambda: RELOAD_MOD())

def EXEC_CLIENT_CODE(data):
    # type: (bytearray) -> None
    # typeID=4：在 client 主线程 exec Python 2.7 代码。
    # 用单独 globals 但暴露 __builtins__，让常见的 print / import 可用。
    code = bytes(data).decode("utf-8", "replace")
    def _do():
        try:
            ns = {"__builtins__": __builtins__, "__name__": "mcdk_exec_client"}
            exec(compile(code, "<mcdk-exec-client>", "exec"), ns)
        except Exception:
            import traceback
            traceback.print_exc()
    _ON_MAIN_THREAD(_do)

def EXEC_SERVER_CODE(data):
    # type: (bytearray) -> None
    # typeID=3：server 端 exec 需要 client→server 自定义事件转发，桥模组目前
    # 没在 server 端启 IPCSystem。先给明确报错，让上层 fallback 到 isClient=true。
    print("[IPCSystem] ExecuteServerCode 暂未实现：桥模组当前只在 client 端跑 IPC。"
          "请使用 ExecuteClientCode（mcp 工具调用时 isClient=true）。")

def RELOAD_GAME_WORLD(_=None):
    # typeID=5：重启本地世界（不退游戏）
    from .Game import RELOAD_WORLD
    _ON_MAIN_THREAD(lambda: RELOAD_WORLD())

def RELOAD_ALL_SHADERS(_=None):
    # typeID=6：重编译全部 shader
    from .Game import RELOAD_SHADERS
    _ON_MAIN_THREAD(lambda: RELOAD_SHADERS())

def RELOAD_SINGLE_SHADER(data):
    # type: (bytearray) -> None
    # typeID=7：ModSDK 没有"单 shader 热重载"公开 API，降级为重编译全部 +
    # 把目标文件名打到日志，便于使用方核对。
    name = bytes(data).decode("utf-8", "replace") if data else ""
    print("[IPCSystem] reload_single_shader: 引擎无单 shader 重载 API，"
          "降级为 RELOAD_SHADERS（请求文件={!r}）".format(name))
    RELOAD_ALL_SHADERS()

def RELOAD_ADDON_AND_GAME_WORLD(_=None):
    # typeID=8：先刷 add-ons，再重启世界。
    from .Game import RELOAD_ADDON, RELOAD_WORLD
    def _do():
        RELOAD_ADDON()
        RELOAD_WORLD()
    _ON_MAIN_THREAD(_do)

_IPCSYSTEM = IPCSystem(GET_DEBUG_IPC_PORT())
_IPCSYSTEM.updateHandlers(
    {
        1: AUTO_RELOAD,            # 内部：保存代码后自动 reload mod 脚本
        3: EXEC_SERVER_CODE,       # cc-panes mcp__execute_mc_code isClient=false
        4: EXEC_CLIENT_CODE,       # cc-panes mcp__execute_mc_code isClient=true
        5: RELOAD_GAME_WORLD,      # cc-panes mcp__reload_mc kind=game
        6: RELOAD_ALL_SHADERS,     # cc-panes mcp__reload_mc kind=all_shaders
        7: RELOAD_SINGLE_SHADER,   # cc-panes mcp__reload_mc kind=single_shader
        8: RELOAD_ADDON_AND_GAME_WORLD,  # cc-panes mcp__reload_mc kind=addon_and_game
    }
)

def ON_CLIENT_INIT():
    global GAME_COMP
    GAME_COMP = clientApi.GetEngineCompFactory().CreateGame(clientApi.GetLevelId())
    _IPCSYSTEM.start()

def ON_CLIENT_EXIT():
    _IPCSYSTEM.close()
