package base

import (
	"hash/fnv"
	"os"
	"reflect"
	"unsafe"
	"runtime"
	"runtime/debug"
	"errors"
	log "github.com/sotter/dovenet/log"
	"sync/atomic"
)

type Hashable interface {
	HashCode() int32
}

func RecoverPrint() {
	var err error
	if r := recover(); r !=nil{
		switch x := r.(type) {
		case string:
			err = errors.New(x)
			break
		case error:
			err = x
			break
		default:
			err = errors.New("Unknown panic")
			break
		}
		debug.PrintStack()
		log.Println("Panic :", err.Error())
	}
}

var  netIdentifier = uint64(1)

func GetNetId() uint64 {
	return atomic.AddUint64(&netIdentifier, 1)
}

const intSize = unsafe.Sizeof(1)

func hashCode(k interface{}) uint32 {
	var code uint32
	h := fnv.New32a()
	switch v := k.(type) {
	case bool:
		h.Write((*((*[1]byte)(unsafe.Pointer(&v))))[:])
		code = h.Sum32()
	case int:
		h.Write((*((*[intSize]byte)(unsafe.Pointer(&v))))[:])
		code = h.Sum32()
	case int8:
		h.Write((*((*[1]byte)(unsafe.Pointer(&v))))[:])
		code = h.Sum32()
	case int16:
		h.Write((*((*[2]byte)(unsafe.Pointer(&v))))[:])
		code = h.Sum32()
	case int32:
		h.Write((*((*[4]byte)(unsafe.Pointer(&v))))[:])
		code = h.Sum32()
	case int64:
		h.Write((*((*[8]byte)(unsafe.Pointer(&v))))[:])
		code = h.Sum32()
	case uint:
		h.Write((*((*[intSize]byte)(unsafe.Pointer(&v))))[:])
		code = h.Sum32()
	case uint8:
		h.Write((*((*[1]byte)(unsafe.Pointer(&v))))[:])
		code = h.Sum32()
	case uint16:
		h.Write((*((*[2]byte)(unsafe.Pointer(&v))))[:])
		code = h.Sum32()
	case uint32:
		h.Write((*((*[4]byte)(unsafe.Pointer(&v))))[:])
		code = h.Sum32()
	case uint64:
		h.Write((*((*[8]byte)(unsafe.Pointer(&v))))[:])
		code = h.Sum32()
	case string:
		h.Write([]byte(v))
		code = h.Sum32()
	case Hashable:
		c := v.HashCode()
		h.Write((*((*[4]byte)(unsafe.Pointer(&c))))[:])
		code = h.Sum32()
	default:
		panic("key not hashable")
	}
	return code
}

func isNil(v interface{}) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	kd := rv.Type().Kind()
	switch kd {
	case reflect.Ptr, reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

func printStack() {
	var buf [4096]byte
	n := runtime.Stack(buf[:], false)
	os.Stderr.Write(buf[:n])
}

func FormatOutput(field1 string, field2 string, total_len int) string {
	//中间至少保留 3个"-"的占位符
	output := make([]byte, total_len)
	len1 := len(field1)
	len2 := len(field2)

	if len1 > 30 {
		field1 = field1[:30]
		len1 = 30
	}
	if len2 > 30 {
		field2 = field2[:30]
		len2 = 30



	}

	j := 0
	for i := 0; i < total_len; i++ {

		if i < len1 {
			output[i] = field1[i]
			continue
		}
		//至少填充3个占位符
		if i < len1 + 3 {
			output[i] = '-'
			continue
		}

		if i < total_len - len2 {
			output[i] = '-'
			continue
		}

		output[i] = field2[j]
		j++
	}

	return  string(output)
}