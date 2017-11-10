// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package net

import (
	"bytes"
	"syscall"
	"testing"
	"unsafe"
)

func TestRawConn(t *testing.T) {
	handler := func(ls *localServer, ln Listener) {
		c, err := ln.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		defer c.Close()
		var b [32]byte
		n, err := c.Read(b[:])
		if err != nil {
			t.Error(err)
			return
		}
		if _, err := c.Write(b[:n]); err != nil {
			t.Error(err)
			return
		}
	}
	ls, err := newLocalServer("tcp")
	if err != nil {
		t.Fatal(err)
	}
	defer ls.teardown()
	if err := ls.buildup(handler); err != nil {
		t.Fatal(err)
	}

	c, err := Dial(ls.Listener.Addr().Network(), ls.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	cc, err := c.(*TCPConn).SyscallConn()
	if err != nil {
		t.Fatal(err)
	}

	var operr error
	data := []byte("HELLO-R-U-THERE")
	err = cc.Write(func(s uintptr) bool {
		var written uint32
		var buf syscall.WSABuf
		buf.Buf = &data[0]
		buf.Len = uint32(len(data))
		operr = syscall.WSASend(syscall.Handle(s), &buf, 1, &written, 0, nil, nil)
		if operr == syscall.EAGAIN {
			return false
		}
		return true
	})
	if err != nil || operr != nil {
		t.Fatal(err, operr)
	}

	var nr int
	var b [32]byte
	err = cc.Read(func(s uintptr) bool {
		var read uint32
		var flags uint32
		var buf syscall.WSABuf
		buf.Buf = &b[0]
		buf.Len = uint32(len(b[:]))
		operr = syscall.WSARecv(syscall.Handle(s), &buf, 1, &read, &flags, nil, nil)
		if operr == syscall.EAGAIN {
			return false
		}
		nr = int(read)
		return true
	})
	if err != nil || operr != nil {
		t.Fatal(err, operr)
	}
	if bytes.Compare(b[:nr], data) != 0 {
		t.Fatalf("got %#v; want %#v", b[:nr], data)
	}

	fn := func(s uintptr) {
		operr = syscall.SetsockoptInt(syscall.Handle(s), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	}
	err = cc.Control(fn)
	if err != nil || operr != nil {
		t.Fatal(err, operr)
	}
	c.Close()
	err = cc.Control(fn)
	if err == nil {
		t.Fatal("should fail")
	}
}

func TestRawConnUDP(t *testing.T) {
	c, err := newLocalPacketListener("udp")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	cc, err := c.(*UDPConn).SyscallConn()
	if err != nil {
		t.Fatal(err)
	}

	var operr error
	fn := func(s uintptr) {
		operr = syscall.SetsockoptInt(syscall.Handle(s), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	}
	err = cc.Control(fn)
	if err != nil || operr != nil {
		t.Fatal(err, operr)
	}
	c.Close()
	err = cc.Control(fn)
	if err == nil {
		t.Fatal("should fail")
	}
}

func TestRawConnListener(t *testing.T) {
	ln, err := newLocalListener("tcp")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	cc, err := ln.(*TCPListener).SyscallConn()
	if err != nil {
		t.Fatal(err)
	}

	called := false
	op := func(uintptr) bool {
		called = true
		return true
	}

	err = cc.Write(op)
	if err == nil {
		t.Error("Write should return an error")
	}
	if called {
		t.Error("Write shouldn't call op")
	}

	called = false
	err = cc.Read(op)
	if err == nil {
		t.Error("Read should return an error")
	}
	if called {
		t.Error("Read shouldn't call op")
	}

	var operr error
	fn := func(s uintptr) {
		var v, l int32
		l = int32(unsafe.Sizeof(v))
		operr = syscall.Getsockopt(syscall.Handle(s), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, (*byte)(unsafe.Pointer(&v)), &l)
	}
	err = cc.Control(fn)
	if err != nil || operr != nil {
		t.Fatal(err, operr)
	}
	ln.Close()
	err = cc.Control(fn)
	if err == nil {
		t.Fatal("Control after Close should fail")
	}
}
