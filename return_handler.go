package martini

import (
	"github.com/codegangsta/inject"
	"net/http"
	"reflect"
)

// ReturnHandler is a service that Martini provides that is called
// when a route handler returns something. The ReturnHandler is
// responsible for writing to the ResponseWriter based on the values
// that are passed into this function.
// ReturnHandler 是 Martini 提供的一个服务，当路由处理有一些东西返回的时候会被调用到。
// 其主要职责是根据传入的值，写入 ResponseWriter
type ReturnHandler func(Context, []reflect.Value)

func defaultReturnHandler() ReturnHandler {
	return func(ctx Context, vals []reflect.Value) {
		rv := ctx.Get(inject.InterfaceOf((*http.ResponseWriter)(nil)))
		res := rv.Interface().(http.ResponseWriter)
		var responseVal reflect.Value
		if len(vals) > 1 && vals[0].Kind() == reflect.Int {
			res.WriteHeader(int(vals[0].Int()))
			responseVal = vals[1]
		} else if len(vals) > 0 {
			responseVal = vals[0]
		}
		if canDeref(responseVal) {
			responseVal = responseVal.Elem()
		}
		if isByteSlice(responseVal) {
			res.Write(responseVal.Bytes())
		} else {
			res.Write([]byte(responseVal.String()))
		}
	}
}

// 判断是否是 byte slice
func isByteSlice(val reflect.Value) bool {
	return val.Kind() == reflect.Slice && val.Type().Elem().Kind() == reflect.Uint8
}

// 判断是否是指针类型或者是借口类型
func canDeref(val reflect.Value) bool {
	return val.Kind() == reflect.Interface || val.Kind() == reflect.Ptr
}
