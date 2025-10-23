package Utils

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// CreateFile crea un archivo vacío en la ruta especificada.
func CreateFile(name string) error {
	// Asegura que el directorio exista
	dir := filepath.Dir(name)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("error al crear el directorio: %v", err)
	}

	// Crea el archivo si no existe
	if _, err := os.Stat(name); os.IsNotExist(err) {
		file, err := os.Create(name)
		if err != nil {
			return fmt.Errorf("error al crear el archivo: %v", err)
		}
		defer file.Close()
	}

	return nil
}

// OpenFile abre un archivo en modo lectura y escritura.
func OpenFile(name string) (*os.File, error) {
	file, err := os.OpenFile(name, os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("error al abrir el archivo: %v", err)
	}
	return file, nil
}

// WriteFile escribe un objeto en un archivo binario en la posición especificada.
func WriteFile(file *os.File, data interface{}, position int64) error {
	if _, err := file.Seek(position, 0); err != nil {
		return fmt.Errorf("error al buscar la posición en el archivo: %v", err)
	}

	if err := binary.Write(file, binary.LittleEndian, data); err != nil {
		return fmt.Errorf("error al escribir el objeto en el archivo: %v", err)
	}

	return nil
}

// ReadFile lee un objeto de un archivo binario en la posición especificada.
func ReadFile(file *os.File, data interface{}, position int64) error {
	if _, err := file.Seek(position, 0); err != nil {
		return fmt.Errorf("error al buscar la posición en el archivo: %v", err)
	}

	if err := binary.Read(file, binary.LittleEndian, data); err != nil {
		return fmt.Errorf("error al leer el objeto del archivo: %v", err)
	}

	return nil
}

func UploadS3(bucket, localPath, s3Key string) (string, error) {
	// Abre el archivo que quieres subir
	file, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("no se pudo abrir el archivo local %s: %v", localPath, err)
	}
	defer file.Close()

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)
	if err != nil {
		return "", fmt.Errorf("fallo al crear la sesión de AWS: %v", err)
	}

	// Crea un uploader
	uploader := s3manager.NewUploader(sess)

	// Sube el archivo
	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(s3Key),
		Body:   file,
	})

	if err != nil {
		return "", fmt.Errorf("fallo al subir el archivo a S3: %v", err)
	}

	// La URL pública del archivo subido
	return result.Location, nil
}
