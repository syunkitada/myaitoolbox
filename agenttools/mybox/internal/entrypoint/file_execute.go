package entrypoint

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// ExecuteFileStream attaches a browser to a server-owned file execution job.
// The job continues after the WebSocket disconnects and can be reattached by
// its run ID. The project and run ID are query parameters because browsers
// cannot set the X-Project header during a WebSocket handshake.
func (s *Server) ExecuteFileStream(c echo.Context) error {
	project := s.fileExecutionProject(c.Request())
	if _, err := s.getAppByProject(c.Request().Context(), project); err != nil {
		return err
	}
	jobID := c.QueryParam("run_id")
	if jobID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "run_id is required")
	}
	job := s.fileExecutions.get(project, jobID)
	if job == nil {
		return echo.NewHTTPError(http.StatusNotFound, "file execution not found")
	}

	conn, err := terminalUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	client := &fileExecutionClient{
		send: make(chan fileExecutionStreamMessage, 64),
		done: make(chan struct{}),
		connClose: func() {
			_ = conn.Close()
		},
	}
	job.attach(client)
	defer job.detach(client)

	go func() {
		for {
			select {
			case message := <-client.send:
				if err := conn.WriteJSON(message); err != nil {
					client.halt()
					return
				}
				if message.Type == "state" && message.Status != fileExecutionRunning {
					client.halt()
					return
				}
			case <-client.done:
				return
			}
		}
	}()
	for {
		if _, _, readErr := conn.ReadMessage(); readErr != nil {
			return nil
		}
	}
}
