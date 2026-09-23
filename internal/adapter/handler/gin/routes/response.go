package routes

import "github.com/gin-gonic/gin"

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Error   string `json:"error"`
	Payload any    `json:"payload"`
}

func ResData(c *gin.Context, code int, message, errStr string, data any) {
	c.JSON(code, Response{Code: code, Message: message, Error: errStr, Payload: data})
}
