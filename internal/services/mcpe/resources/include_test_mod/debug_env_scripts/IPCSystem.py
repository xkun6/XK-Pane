# -*- coding: utf-8 -*-
import socket
import threading
import collections
import mod.client.extraClientApi as clientApi
from .Config import GET_DEBUG_IPC_PORT, GET_DEBUG_RUN_ID

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

# Route A 最小验证：反向日志帧类型（与后端 mcdkbridge.PacketLogLine 一致）。
LOG_TYPE_ID = 100
# 连接后第一帧，payload=runId（与后端 mcdkbridge.PacketHello 一致）。
HELLO_TYPE_ID = 101

class IPCSystem:
    def __init__(self, port=None):
        # type: (int | None) -> None
        self.port = port
        self.sock = None
        self.mLock = threading.Lock()
        self.handers = {}
        # 反向日志发送：有界队列 + 独立发送线程。
        # 为什么不在 STD_OUT_WRAPPER.write 里直接 sendall——那是游戏主线程，socket 发送缓冲
        # 满会阻塞主线程拖卡游戏。改为入队（非阻塞、满则丢最旧），由 _senderLoop 异步发。
        # _connected 前不入队（丢弃）：连上那刻起才 tee，避免回放 stdout 已显示过的早期行
        # 在后端与 stdout 来源重复（MC Studio 关着的常见场景）。代价是损失连上前的少量启动行。
        self._sendQ = collections.deque(maxlen=4000)
        self._sendCv = threading.Condition()
        self._connected = False

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
        self._connected = False
        with self._sendCv:
            self._sendCv.notify_all()  # 唤醒发送线程，让它看到 sock=None 后退出
        if sock:
            sock.close()

    def enqueueLog(self, payload):
        # type: (bytes) -> None
        # 把一帧日志 payload 入队（非阻塞、满则 deque 自动丢最旧），唤醒发送线程。
        # 未连上前直接丢弃（见 _connected 说明）。绝不在此 print / sendall（游戏主线程调用）。
        if not self._connected:
            return
        try:
            with self._sendCv:
                self._sendQ.append(payload)
                self._sendCv.notify()
        except Exception:
            pass

    def _senderLoop(self):
        # 独立发送线程：阻塞式 sendall 只发生在这里，不波及游戏主线程。
        while 1:
            payload = None
            with self._sendCv:
                while not self._sendQ:
                    if not self.sock:
                        return
                    self._sendCv.wait(0.5)
                if not self.sock:
                    return
                payload = self._sendQ.popleft()
            if payload is not None:
                self.sendFrame(LOG_TYPE_ID, payload)

    def sendFrame(self, typeID, payload):
        # type: (int, bytes) -> None
        # 反向发一帧 [2B TypeID][4B DataLength][payload]（与后端 protocol.go Decode 对称）。
        # 由 _senderLoop（发送线程）与连接线程的一次性 hello 调用——hello 发生在发送线程启动前，
        # 无并发，故无需 mLock，sendall 阻塞也只阻塞调用者所在线程。
        # 未连/出错一律静默吞，绝不 print（会经 STD_OUT_WRAPPER 递归回来）。
        sock = self.sock
        if not sock:
            return
        try:
            n = len(payload)
            frame = bytearray(6)
            frame[0] = (typeID >> 8) & 0xFF
            frame[1] = typeID & 0xFF
            frame[2] = (n >> 24) & 0xFF
            frame[3] = (n >> 16) & 0xFF
            frame[4] = (n >> 8) & 0xFF
            frame[5] = n & 0xFF
            frame.extend(bytearray(payload))
            sock.sendall(bytes(frame))
        except Exception:
            pass

    def _threadListenLoop(self):
        with self.mLock:
            self.sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock = self.sock
        sock.connect(("localhost", self.port))
        # 先发 hello 帧（runId 归属），再开日志闸——保证 hello 是本连接第一帧，
        # XK-Pane 据此把本连接后续日志归到对应 run（多开客户端互不串台）。
        runId = GET_DEBUG_RUN_ID()
        if runId:
            self.sendFrame(HELLO_TYPE_ID, runId.encode("utf-8") if isinstance(runId, unicode) else runId)
        # 连上后开闸 + 起发送线程，自此 stdout 每行经队列异步发往 XK-Pane。
        self._connected = True
        threading.Thread(target=self._senderLoop).start()
        self.enqueueLog(b"[MCDK-BRIDGE] log tee online")  # 连通标记（诊断用）
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

def SEND_LOG_LINE(line):
    # type: (str) -> None
    # 把一行 stdout 经 IPC 桥 tee 回 XK-Pane（PacketLogLine=100）：非阻塞入队，发送线程异步发。
    # 静默吞一切异常、绝不 print（由 STD_OUT_WRAPPER.write 调用，print 会无限递归）。
    try:
        if isinstance(line, unicode):
            data = line.encode("utf-8", "replace")
        else:
            data = line
        _IPCSYSTEM.enqueueLog(data)
    except Exception:
        pass

def ON_CLIENT_INIT():
    global GAME_COMP
    GAME_COMP = clientApi.GetEngineCompFactory().CreateGame(clientApi.GetLevelId())
    _IPCSYSTEM.start()

def ON_CLIENT_EXIT():
    _IPCSYSTEM.close()
