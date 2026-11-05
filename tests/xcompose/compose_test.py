import ctypes
import os
import pathlib
import shutil
import signal
import subprocess
import sys
import tempfile
import time
import unittest

COMPOSE_FILE = pathlib.Path(__file__).resolve().parents[2] / "xcompose" / "XCompose"

DEAD_KEYS = {
    "dead_acute": (0xFE51, "'", dict(zip("aeiouyAEIOUYcC", "áéíóúýÁÉÍÓÚÝçÇ"))),
    "dead_diaeresis": (0xFE57, '"', dict(zip("aeiouyAEIOU", "äëïöüÿÄËÏÖÜ"))),
    "dead_circumflex": (0xFE52, "^", dict(zip("aeiouAEIOU", "âêîôûÂÊÎÔÛ"))),
    "dead_grave": (0xFE50, "`", dict(zip("aeiouAEIOU", "àèìòùÀÈÌÒÙ"))),
    "dead_tilde": (0xFE53, "~", dict(zip("anoANO", "ãñõÃÑÕ"))),
}
MULTI_KEY = 0xFF20
COMPOSED = 2


def windows_sequences():
    for name, (dead, symbol, accented) in DEAD_KEYS.items():
        for second_name, (second, second_symbol, _) in DEAD_KEYS.items():
            yield name, second_name, [dead, second], symbol + second_symbol
        for code in range(0x20, 0x7F):
            key = chr(code)
            yield name, key, [dead, code], symbol if key == " " else accented.get(key, symbol + key)


class Composer:
    def __init__(self, library, compose_file):
        self.library = library
        for name in ("xkb_context_new", "xkb_compose_table_new_from_locale", "xkb_compose_state_new"):
            getattr(library, name).restype = ctypes.c_void_p
        library.xkb_context_new.argtypes = [ctypes.c_int]
        library.xkb_compose_table_new_from_locale.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_int]
        library.xkb_compose_state_new.argtypes = [ctypes.c_void_p, ctypes.c_int]
        library.xkb_compose_state_feed.argtypes = [ctypes.c_void_p, ctypes.c_uint32]
        library.xkb_compose_state_get_status.argtypes = [ctypes.c_void_p]
        library.xkb_compose_state_get_utf8.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_size_t]
        library.xkb_compose_state_reset.argtypes = [ctypes.c_void_p]
        previous = os.environ.get("XCOMPOSEFILE")
        os.environ["XCOMPOSEFILE"] = str(compose_file)
        try:
            table = library.xkb_compose_table_new_from_locale(library.xkb_context_new(0), b"en_US.UTF-8", 0)
        finally:
            if previous is None:
                del os.environ["XCOMPOSEFILE"]
            else:
                os.environ["XCOMPOSEFILE"] = previous
        if not table:
            raise RuntimeError(f"libxkbcommon could not load {compose_file} with the en_US.UTF-8 system table")
        self.state = library.xkb_compose_state_new(table, 0)

    def compose(self, keysyms):
        self.library.xkb_compose_state_reset(self.state)
        for keysym in keysyms:
            self.library.xkb_compose_state_feed(self.state, keysym)
        if self.library.xkb_compose_state_get_status(self.state) != COMPOSED:
            return None
        buffer = ctypes.create_string_buffer(64)
        self.library.xkb_compose_state_get_utf8(self.state, buffer, len(buffer))
        return buffer.value.decode()


class LibxkbcommonComposeTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        if not sys.platform.startswith("linux"):
            raise unittest.SkipTest("the Compose file is Linux only")
        try:
            library = ctypes.CDLL("libxkbcommon.so.0")
        except OSError:
            raise unittest.SkipTest("libxkbcommon is not installed")
        cls.composer = Composer(library, COMPOSE_FILE)

    def test_each_dead_key_sequence_types_what_windows_types(self):
        for name, key, keysyms, expected in windows_sequences():
            with self.subTest(dead_key=name, key=key):
                self.assertEqual(self.composer.compose(keysyms), expected)

    def test_the_system_compose_sequences_stay_available(self):
        self.assertEqual(self.composer.compose([MULTI_KEY, ord("o"), ord("c")]), "©")


class IBusComposeTest(unittest.TestCase):
    IBUS_KEEPS_ONE_SYMBOL = {"dead_circumflex", "dead_grave", "dead_tilde"}

    @classmethod
    def setUpClass(cls):
        if not sys.platform.startswith("linux"):
            raise unittest.SkipTest("the Compose file is Linux only")
        daemon, session = shutil.which("ibus-daemon"), shutil.which("dbus-run-session")
        if not daemon or not session:
            raise unittest.SkipTest("ibus-daemon or dbus-run-session is not installed")
        try:
            import gi

            gi.require_version("IBus", "1.0")
            from gi.repository import GLib, IBus
        except (ImportError, ValueError):
            raise unittest.SkipTest("the IBus Python bindings are not installed")
        cls.GLib, cls.IBus = GLib, IBus
        cls.home = tempfile.TemporaryDirectory()
        home = pathlib.Path(cls.home.name)
        (home / ".XCompose").symlink_to(COMPOSE_FILE)
        runtime = home / "runtime"
        runtime.mkdir(mode=0o700)
        socket = home / "ibus.socket"
        environment = {
            key: value
            for key, value in os.environ.items()
            if key not in {"DISPLAY", "WAYLAND_DISPLAY", "DBUS_SESSION_BUS_ADDRESS", "IBUS_ADDRESS"}
        }
        environment.update(
            HOME=str(home),
            XDG_CONFIG_HOME=str(home / ".config"),
            XDG_CACHE_HOME=str(home / ".cache"),
            XDG_DATA_HOME=str(home / ".local/share"),
            XDG_RUNTIME_DIR=str(runtime),
            GSETTINGS_BACKEND="memory",
        )
        cls.daemon = subprocess.Popen(
            [session, "--", daemon, "--panel=disable", "--emoji-extension=disable", "--config=disable",
             "--cache=none", f"--address=unix:path={socket}"],
            env=environment, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True,
        )
        cls.addClassCleanup(cls.stop_daemon)
        deadline = time.monotonic() + 30
        while not socket.exists():
            if time.monotonic() > deadline or cls.daemon.poll() is not None:
                raise RuntimeError("ibus-daemon did not start")
            time.sleep(0.05)
        os.environ["IBUS_ADDRESS"] = f"unix:path={socket}"
        IBus.init()
        cls.bus = IBus.Bus.new()
        cls.wait_for(cls.bus.is_connected, "IBus connection")
        cls.context = cls.bus.create_input_context("dotfiles-xcompose-test")
        cls.context.set_capabilities(IBus.Capabilite.FOCUS)
        cls.committed = []
        cls.context.connect("commit-text", lambda _, text: cls.committed.append(text.get_text()))
        cls.context.focus_in()
        cls.bus.set_global_engine("xkb:us:intl:eng")
        cls.wait_for(lambda: (cls.context.get_engine() or IBus.EngineDesc()).get_name() == "xkb:us:intl:eng",
                     "the xkb:us:intl:eng engine")

    @classmethod
    def stop_daemon(cls):
        os.environ.pop("IBUS_ADDRESS", None)
        try:
            os.killpg(cls.daemon.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        cls.daemon.wait(timeout=30)
        cls.home.cleanup()

    @classmethod
    def wait_for(cls, condition, what, seconds=30):
        deadline = time.monotonic() + seconds
        while not condition():
            if time.monotonic() > deadline:
                raise RuntimeError(f"timed out waiting for {what}")
            cls.GLib.MainContext.default().iteration(False)
            time.sleep(0.01)

    def type(self, keyvals):
        self.context.reset()
        self.committed.clear()
        for keyval in keyvals:
            self.context.process_key_event(keyval, 0, 0)
            self.context.process_key_event(keyval, 0, self.IBus.ModifierType.RELEASE_MASK)
        try:
            self.wait_for(lambda: self.committed, "committed text", seconds=2)
        except RuntimeError:
            return None
        return "".join(self.committed)

    def test_each_dead_key_sequence_types_what_windows_types(self):
        for name, key, keyvals, expected in windows_sequences():
            if key == name and name in self.IBUS_KEEPS_ONE_SYMBOL:
                expected = expected[0]
            with self.subTest(dead_key=name, key=key):
                self.assertEqual(self.type(keyvals), expected)

    def test_the_system_compose_sequences_stay_available(self):
        self.assertEqual(self.type([MULTI_KEY, ord("o"), ord("c")]), "©")


if __name__ == "__main__":
    unittest.main()
