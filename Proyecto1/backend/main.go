package main

// Imports
import (
	"backend/Analizador"
	"bytes"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

type CommandRequest struct {
	Command string `json:"command"`
}

type CommandResponse struct {
	Output string `json:"output"`
}

func main() {
	app := fiber.New()
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Content-Type",
	}))

	// Healthcheck simple
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Ejecuta 1..N líneas de comandos (y comentarios) en un solo request
	app.Post("/execute", func(c *fiber.Ctx) error {
		var req CommandRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(CommandResponse{
				Output: "Error: Petición inválida",
			})
		}

		script := strings.ReplaceAll(req.Command, "\r\n", "\n") // normaliza CRLF
		lines := strings.Split(script, "\n")

		var out bytes.Buffer

		for _, raw := range lines {
			line := strings.TrimSpace(raw)
			if line == "" {

				out.WriteString("\n")
				continue
			}
			if strings.HasPrefix(line, "#") {

				out.WriteString(line + "\n")
				continue
			}

			cmd, params := Analizador.GetInput(line)
			if cmd == "" {
				out.WriteString("Error: línea no reconocida\n")
				continue
			}

			result := Analizador.AnalyzerCommand(cmd, params)

			if !strings.HasSuffix(result, "\n") {
				result += "\n"
			}
			out.WriteString(result)
		}

		output := strings.TrimRight(out.String(), "\n") + "\n"
		if strings.TrimSpace(output) == "" {
			output = "No se ejecutó ningún comando\n"
		}

		return c.JSON(CommandResponse{Output: output})
	})

	fmt.Println("🚀 Backend listo en http://localhost:3001")
	_ = app.Listen(":3001")
}
