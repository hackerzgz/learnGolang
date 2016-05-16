package main

import (
    "os"
    "errors"
    "io"
    "sync"
    "sync/atomic"
)

/*
 * rsn : 最后被读取的数据块序号
 * wsn : 最后被写入的数据块序号
 * woffset : 写入偏移量
 * roffset : 读取偏移量
 */

// Data is []byte
type Data []byte

// DataFile 数据文件的接口类型
type DataFile interface {
    // read a data bolck
    Read() (rsn int64, d Data, err error)
    // write a data bolck
    Write(d Data) (wsn int64, err error)
    // Get the value of rsn
    Rsn() int64
    // Get the value of wsn
    Wsn() int64
    // Get the length if data bolck
    DataLen() uint32
}


type myDataFile struct {
    f       *os.File        // File
    fmutex  sync.RWMutex    // RWMutex for File (control the f)
    woffset int64           // write offset
    roffset int64           // read offset
    wmutex  sync.Mutex      // write mutex (control the woffset)
    rmutex  sync.Mutex      // read mutex (control the roffset)
    dataLen uint32          // length of data block (uint32 is easy to trans int or int64)
    rcond   *sync.Cond      // conditional variable
}

// NewDataFile init myDataFile
func NewDataFile(path string, dataLen uint32) (DataFile, error)  {
    f, err := os.Create(path)
    if err != nil {
        return nil, err
    }
    if dataLen == 0 {
        return nil, errors.New("Invalid data length!")
    }
    // Other variable will be initialized to the default value
    df := &myDataFile{f: f, dataLen: dataLen}
    // init myDataFile conditional variable
    df.rcond = sync.NewCond(df.fmutex.RLocker())
    return df, nil
}

func (df *myDataFile) DataLen() uint32 {
    return df.dataLen
}

func (df *myDataFile) Read() (rsn int64, d Data, err error) {
    // read and update read offset
    var offset int64
    // for --> get the offset safely
    for {
        // write a 64byte data into 32byte machine
        offset = atomic.LoadInt64(&df.roffset)
        if atomic.CompareAndSwapInt64(&df.roffset, offset,
         (offset + int64(df.dataLen))) {
            break
        }
    }
    
    // Read a Data Bolck
    rsn = offset / int64(df.dataLen)
    bytes := make([]byte, df.dataLen)
    df.fmutex.Lock()
    defer df.fmutex.Unlock()
    for {
        _, err = df.f.ReadAt(bytes, offset)
        if err != nil {
            if err == io.EOF {
                df.rcond.Wait()
                continue
            }
            return
        }
        // Read the Data Successful
        d = bytes
        return
    }
}

func (df *myDataFile) Write(d Data) (wsn int64, err error)  {
    // read and update write offset
    var offset int64
    // for --> get the offset safely
    for {
        offset := atomic.LoadInt64(&df.woffset)
        if atomic.CompareAndSwapInt64(&df.woffset, offset, (offset + int64(df.dataLen))) {
            break
        }
    }
    
    // Write a Data Bolck
    wsn = offset / int64(df.dataLen)
    var bytes []byte
    if len(d) > int(df.dataLen) {
        bytes = d[0:df.dataLen]
    }  else {
        bytes = d
    }
    df.fmutex.Lock()
    defer df.fmutex.Unlock()
    _, err = df.f.Write(bytes)
    df.rcond.Signal()
    return
}

func (df *myDataFile) Rsn() int64 {
    // atomic Load
    offset := atomic.LoadInt64(&df.roffset)
    return offset / int64(df.dataLen)
}

func (df *myDataFile) Wsn() int64 {
    // atomic Load
    offset := atomic.LoadInt64(&df.woffset)
    return offset / int64(df.dataLen)
}