package logger

import (
    "log"
    "os"
)

var Logger = log.New(
    os.Stdout,
    "[GHOST] ",
    log.Ldate|log.Ltime|log.Lmicroseconds,
)
