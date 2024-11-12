package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Estructura del MBR
type MBR struct {
	MbrTamano        int64         // Tamaño total del disco en bytes
	MbrFechaCreacion [19]byte      // Fecha y hora de creación del disco
	MbrDskSignature  int32         // Número random que identifica de forma única a cada disco
	DskFit           byte          // Tipo de ajuste de la partición: 'B', 'F', o 'W'
	Partitions       [4]Partition1 // Arreglo con información de las 4 particiones
}

type Disk struct {
	Size       int64       `json:"size"`
	Unit       string      `json:"unit"`
	Fit        string      `json:"fit"`
	Path       string      `json:"path"`
	Partitions []Partition `json:"particiones"`
}

// -------------------------------------MKDIR-DISCOS--------------------------------

func parseMkdirCommand(command2 string) (size int64, unit, path, fit string, err error) {
	parts := strings.Fields(strings.ToLower(command2))
	cleanedCommand := strings.SplitN(command2, "#", 2)[0]
	cleanedCommand = strings.TrimSpace(cleanedCommand)
	cleanedCommand = strings.ToLower(cleanedCommand)

	// Valores por defecto
	fit = "ff" // Valor por defecto es First Fit
	unit = "m" // Valor por defecto es MB

	// Mapa para validar los parámetros permitidos
	validParams := map[string]bool{
		"-size":  true,
		"-unit":  true,
		"-path":  true,
		"-fit":   true,
		"mkdisk": true,
	}

	// Variables para marcar si los parámetros han sido reconocidos
	foundSize, foundPath, foundMkdisk := false, false, false

	for _, part := range parts {
		if strings.HasPrefix(part, "-size=") {
			sizeStr := strings.TrimPrefix(part, "-size=")
			fmt.Sscanf(sizeStr, "%d", &size)
			foundSize = true
		} else if strings.HasPrefix(part, "-unit=") {
			unit = strings.TrimPrefix(part, "-unit=")
			//foundUnit = true
		} else if strings.HasPrefix(part, "-path=") {
			path = strings.TrimPrefix(part, "-path=")
			// Manejo de comillas alrededor de la ruta
			if len(path) > 0 && path[0] == '"' && path[len(path)-1] == '"' {
				path = path[1 : len(path)-1]
			} else {
				re := regexp.MustCompile(`-path="([^"]+)"`)
				matches := re.FindStringSubmatch(cleanedCommand)
				if len(matches) > 1 {
					path = matches[1]
				}
			}
			foundPath = true
		} else if strings.HasPrefix(part, "-fit=") {
			fit = strings.TrimPrefix(part, "-fit=")
			//foundFit = true
		} else if strings.HasPrefix(part, "mkdisk") {
			foundMkdisk = true
		} else {
			// Validación de parámetro no reconocido
			paramName := strings.Split(part, "=")[0]
			if !validParams[paramName] {
				err = fmt.Errorf("parámetro inválido: %s", part)
				return
			}
		}
	}

	// Validaciones adicionales
	if !foundSize {
		err = fmt.Errorf("tamaño de disco no especificado o igual a cero")
		return
	}
	if !foundPath {
		err = fmt.Errorf("ruta del disco no especificada")
		return
	}
	// if !foundUnit {
	// 	err = fmt.Errorf("unidad del disco no especificada")
	// 	return
	// }
	// if !foundFit {
	// 	err = fmt.Errorf("Fit del disco no especificado")
	// 	return
	//}
	if !foundMkdisk {
		err = fmt.Errorf("No se pudo crear el disco ")
		return
	}
	if !strings.Contains("bf,ff,wf", fit) {
		err = fmt.Errorf("valor de ajuste inválido: %s", fit)
		return
	}
	if !strings.Contains("k,m", unit) {
		err = fmt.Errorf("valor de unidad inválido: %s", unit)
		return
	}

	return
}

func processMkdirCommand(command2 string) (Disk, error) { // Cambiado para devolver fit y error
	size, unit, path, fit, err := parseMkdirCommand(command2)
	//fmt.Print("\n" + path + " " + fit + " " + unit + " " + string(size) + "54")
	if err != nil {
		return Disk{}, err
	}

	// Convertir tamaño a bytes según la unidad
	if unit == "m" {
		size *= 1024 * 1024
	} else if unit == "k" {
		size *= 1024
	}

	// Convertir fit de string a byte
	var fitByte byte
	switch fit {
	case "bf":
		fitByte = 'b'
	case "ff":
		fitByte = 'f'
	case "wf":
		fitByte = 'w'
	default:
		fitByte = 'f'
	}

	crearDisco(path, size, fitByte)

	return Disk{
		Size: size,
		Unit: unit,
		Fit:  string(fitByte),
		Path: path,
	}, nil
}

// Función para crear un nuevo disco con un MBR
func crearDisco(path string, size int64, fit byte) {
	var messages []string
	// Crear los directorios necesarios para la ruta si no existen
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		messages = append(messages, fmt.Sprintf("Error al crear los directorios necesarios: %s", err))
		fmt.Println("Error al crear los directorios necesarios:", err)
		return
	}

	// Crear archivo binario para el disco
	file, err := os.Create(path)
	if err != nil {
		messages = append(messages, fmt.Sprintf("Error al crear el archivo del disco: %s", err))
		fmt.Println("Error al crear el archivo del disco:", err)
		return
	}
	defer file.Close()

	// Llenar el archivo con ceros para simular el espacio del disco
	if err := file.Truncate(size); err != nil {
		messages = append(messages, fmt.Sprintf("Error al ajustar el tamaño del archivo: %s", err))
		fmt.Println("Error al ajustar el tamaño del archivo:", err)
		return
	}

	// Formatear la fecha de creación
	fechaCreacion := time.Now().Format("2006-01-02 15:04:05")

	// Convertir la fecha de creación a un byte array de longitud 19
	var fechaCreacionBytes [19]byte
	copy(fechaCreacionBytes[:], fechaCreacion)

	// Crear MBR con valores iniciales
	mbr := MBR{
		MbrTamano:        size,
		MbrFechaCreacion: fechaCreacionBytes,
		MbrDskSignature:  int32(time.Now().UnixNano()), // Generar un número random
		DskFit:           fit,
	}

	// Escribir el MBR en el inicio del archivo
	if err := binary.Write(file, binary.LittleEndian, mbr); err != nil {
		messages = append(messages, fmt.Sprintf("Error al escribir el MBR en el archivo: %s", err))
		fmt.Println("Error al escribir el MBR en el archivo:", err)
		return
	}
}

// -------------------------------------RMDISK-DISCOS--------------------------------
// Analiza el comando rmdisk y extrae el parámetro de la ruta.
func parseRmDiskCommand(command2 string) (path string, err error) {
	parts := strings.Fields(strings.ToLower(command2))
	cleanedCommand := strings.SplitN(command2, "#", 2)[0] // Ignorar comentarios si existen
	cleanedCommand = strings.TrimSpace(cleanedCommand)
	cleanedCommand = strings.ToLower(cleanedCommand)
	//fmt.Println("ComandoRM:", cleanedCommand)
	//fmt.Println("PartsRM:", parts)

	// Buscar la parte que empieza con "-path="
	for _, part := range parts {
		if strings.HasPrefix(part, "-path=") {
			path = strings.TrimPrefix(part, "-path=")
			// Manejo de comillas alrededor de la ruta
			if len(path) > 0 && path[0] == '"' && path[len(path)-1] == '"' {
				path = path[1 : len(path)-1] // Remover comillas al inicio y al final
			}
			break
		} else {
			// Intentar capturar la ruta con expresión regular si no se encuentra de forma directa
			re := regexp.MustCompile(`-path="([^"]+)"`)
			matches := re.FindStringSubmatch(cleanedCommand)
			if len(matches) > 1 {
				path = matches[1]
				break
			}
		}
	}

	// Verificar si se encontró la ruta
	if path == "" {
		err = fmt.Errorf("ruta no especificada en el comando rmdisk")
		return
	}

	return path, nil
}
func deleteDisk(path string) error {
	// Verifica si el archivo existe
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("el archivo '%s' no existe", path)
	}

	// Mostrar confirmación de eliminación
	fmt.Printf("¿Estás seguro de que quieres eliminar el archivo '%s'? (s/n): ", path)
	var response string
	// Escanear y procesar la respuesta del usuario
	_, err := fmt.Scanln(&response)
	if err != nil {
		return fmt.Errorf("error al leer la respuesta: %v", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "s" {
		return fmt.Errorf("operación cancelada por el usuario")
	}

	// Intentar eliminar el archivo
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("error al eliminar el archivo: %v", err)
	}

	fmt.Printf("Archivo '%s' eliminado con éxito.\n", path)
	return nil
}

// Lee el MBR presente en el disco
func readMBR(diskPath string) (MBR, error) {
	file, err := os.Open(diskPath)
	if err != nil {
		return MBR{}, fmt.Errorf("error al abrir el archivo del disco: %v", err)
	}
	defer file.Close()

	var mbr MBR
	mbrData := make([]byte, binary.Size(mbr))
	_, err = file.ReadAt(mbrData, 0)
	if err != nil {
		return MBR{}, fmt.Errorf("error al leer el MBR del disco: %v", err)
	}

	buffer := bytes.NewBuffer(mbrData)
	if err := binary.Read(buffer, binary.LittleEndian, &mbr); err != nil {
		return MBR{}, fmt.Errorf("error al decodificar el MBR: %v", err)
	}

	return mbr, nil
}

// Leer e imprimir la información del MBR en el disco
func PrintMBR(path string) {
	// Abrir el archivo del disco
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Error al abrir el archivo del disco:", err)
		return
	}
	defer file.Close()

	// Leer el MBR existente
	var mbr MBR
	if err := binary.Read(file, binary.LittleEndian, &mbr); err != nil {
		fmt.Println("Error al leer el MBR:", err)
		return
	}

	fmt.Printf("\nInformación del MBR en el disco '%s':\n", path)
	fmt.Println("------------------------------------------------")
	fmt.Printf("Tamaño del disco: %d bytes\n", mbr.MbrTamano)
	fmt.Printf("Fecha de creación: %s\n", string(mbr.MbrFechaCreacion[:]))
	fmt.Printf("Firma del disco: %d\n", mbr.MbrDskSignature)
	fmt.Printf("Ajuste de partición: %c\n", mbr.DskFit)
	fmt.Println("------------------------------------------------")
}
