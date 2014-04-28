// Package martini is a powerful package for quickly writing modular web applications/services in Golang.
//
// For a full guide visit http://github.com/go-martini/martini
//
//  package main
//
//  import "github.com/go-martini/martini"
//
//  func main() {
//    m := martini.Classic()
//
//    m.Get("/", func() string {
//      return "Hello world!"
//    })
//
//    m.Run()
//  }
package martini

import (
	"log"
	"net/http"
	"os"
	"reflect"

	"github.com/codegangsta/inject"
)

// Martini represents the top level web application. inject.Injector methods can be invoked to map services on a global level.
type Martini struct {
	inject.Injector
	// 通过 martini.Use 添加
	handlers []Handler
	// 通过 martini.Action 添加
	action Handler
	logger *log.Logger
}

// New creates a bare bones Martini instance. Use this method if you want to have full control over the middleware that is used.
// 生成一个完全由自己选择的中间件组成的Martini实例
func New() *Martini {
	m := &Martini{Injector: inject.New(), action: func() {}, logger: log.New(os.Stdout, "[martini] ", 0)}
	// Map 了默认的 log.Logger 和 ReturnHandler 对象
	m.Map(m.logger)
	// 注意 route.go 中 func (r *routeContext) run()
	m.Map(defaultReturnHandler())
	return m
}

// Handlers sets the entire middleware stack with the given Handlers. This will clear any current middleware handlers.
// Will panic if any of the handlers is not a callable function
// 清空当前中间件Handler列表，取而代之的是给定的列表，如果列表中的Handler不是可调函数，则panic
func (m *Martini) Handlers(handlers ...Handler) {
	m.handlers = make([]Handler, 0)
	for _, handler := range handlers {
		m.Use(handler)
	}
}

// Action sets the handler that will be called after all the middleware has been invoked. This is set to martini.Router in a martini.Classic().
// Action 设置了在所有的中间件被调用完之后再做调用的处理方法，在 martini.Classic() 中被设置成了路由器
func (m *Martini) Action(handler Handler) {
	validateHandler(handler)
	m.action = handler
}

// Use adds a middleware Handler to the stack. Will panic if the handler is not a callable func. Middleware Handlers are invoked in the order that they are added.
// 将中间件的handler添加到列表中，按照添加的顺序来调用函数。
func (m *Martini) Use(handler Handler) {
	validateHandler(handler)

	m.handlers = append(m.handlers, handler)
}

// ServeHTTP is the HTTP Entry point for a Martini instance. Useful if you want to control your own HTTP server.
// ServeHTTP 是http服务的起始点，可以通过其实现自己控制的http服务
func (m *Martini) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	m.createContext(res, req).run()
}

// Run the http server. Listening on os.GetEnv("PORT") or 3000 by default.
// 运行 http 服务，监听 os.GetEnv("PORT") 端口，默认设置为 3000
func (m *Martini) Run() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	host := os.Getenv("HOST")

	logger := m.Injector.Get(reflect.TypeOf(m.logger)).Interface().(*log.Logger)

	logger.Println("listening on " + host + ":" + port)
	logger.Fatalln(http.ListenAndServe(host+":"+port, m))
}

// 创建 *context 对象
func (m *Martini) createContext(res http.ResponseWriter, req *http.Request) *context {
	c := &context{inject.New(), m.handlers, m.action, NewResponseWriter(res), 0}
	c.SetParent(m)
	c.MapTo(c, (*Context)(nil))
	c.MapTo(c.rw, (*http.ResponseWriter)(nil))
	c.Map(req)
	return c
}

// ClassicMartini represents a Martini with some reasonable defaults. Embeds the router functions for convenience.
type ClassicMartini struct {
	*Martini
	Router
}

// Classic creates a classic Martini with some basic default middleware - martini.Logger, martini.Recovery and martini.Static.
// Classic also maps martini.Routes as a service.
// Classic 实现了一些默认的基本中间件，如果想自定义可以通过New函数实现。
// 默认使用martini.Routes作为路由服务
func Classic() *ClassicMartini {
	r := NewRouter()
	m := New()
	// 通过 Use 方法，将中间件添加到 martini.handlers 中
	m.Use(Logger())
	m.Use(Recovery())
	m.Use(Static("public"))
	m.MapTo(r, (*Routes)(nil))
	// Router.Handle 是路由的入口. 作为 martini.Handler 被使用
	// 将 Router.Handle 添加到 m.action
	// 因为通过 Use 和 Action 添加的 Handler 添加到不同的 handlers 里
	// 所以通过 Use 添加的 Handler 无论在什么地方添加，都会比 Action 添加的 Handler 先执行
	m.Action(r.Handle)
	return &ClassicMartini{m, r}
}

// Handler can be any callable function. Martini attempts to inject services into the handler's argument list.
// Martini will panic if an argument could not be fullfilled via dependency injection.
// Handler 可以是一切可调用的函数，Martini 尝试注入一些服务到函数的参数列表中。
// 如果参数不能实现依赖注入，则将 panic
type Handler interface{}

// 判断 handler 得reflect.Type 是否为 Func 类型，如果不是将 panic
func validateHandler(handler Handler) {
	if reflect.TypeOf(handler).Kind() != reflect.Func {
		panic("martini handler must be a callable func")
	}
}

// Context represents a request context. Services can be mapped on the request level from this interface.
type Context interface {
	inject.Injector
	// Next is an optional function that Middleware Handlers can call to yield the until after
	// the other Handlers have been executed. This works really well for any operations that must
	// happen after an http request
	Next()
	// Written returns whether or not the response for this context has been written.
	Written() bool
}

type context struct {
	inject.Injector
	handlers []Handler
	action   Handler
	rw       ResponseWriter
	index    int
}

// 根据当前的索引来返回Handler，如果当前索引为最后一个，则返回Action
func (c *context) handler() Handler {
	if c.index < len(c.handlers) {
		return c.handlers[c.index]
	}
	if c.index == len(c.handlers) {
		return c.action
	}
	panic("invalid index for context handler")
}

// 执行handler列表中的下一个handler
func (c *context) Next() {
	c.index += 1
	c.run()
}

// 判断 response 是否已经被写入了
func (c *context) Written() bool {
	return c.rw.Written()
}

// 执行当前所有的 handlers，同时索引指向下一个
func (c *context) run() {
	for c.index <= len(c.handlers) {
		_, err := c.Invoke(c.handler())
		if err != nil {
			panic(err)
		}
		c.index += 1

		// 如果 response status 为 0，那么立即停止执行 handlers
		if c.Written() {
			return
		}
	}
}
