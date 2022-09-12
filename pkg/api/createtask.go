package api

import (
	"io/ioutil"
	"net/http"

	"github.com/huo-ju/dfserver/pkg/data"
	"github.com/huo-ju/dfserver/pkg/rabbitmq"
	"github.com/labstack/echo"
	"google.golang.org/protobuf/proto"
)

func CreateTask(amqpQueue *rabbitmq.AmqpQueue) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Echo().Logger.Info("CreateTask")
		priority := uint8(1)

		bodyBytes, err := ioutil.ReadAll(c.Request().Body)
		if err == nil {
			task, err := data.JsonToTask(&bodyBytes)
			if err == nil {
				outputcount := len(task.OutputList)
				nextinput := task.InputList[outputcount]
				body, err := proto.Marshal(task)
				if err != nil {
					return c.JSON(http.StatusInternalServerError, err)
				}
				err = amqpQueue.PublishExchangePriority(*nextinput.Name, "all", body, priority)
				if err != nil {
					return c.JSON(http.StatusInternalServerError, err)
				}
			} else {
				return c.JSON(http.StatusInternalServerError, err)
			}

			return c.JSON(http.StatusOK, &map[string]string{"TaskId": *task.TaskId})
		} else {
			return c.JSON(http.StatusInternalServerError, err)
		}
	}
}
