# mbuffer

## Introduction

`mbuffer` is a memory pool which preserves memory in `sync.Pool` to improve malloc performance.

The usage is quite simple: call `mbuffer.Malloc` directly, and don't forget to `mbuffer.Free` it! 
