package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Estructura de una partición
type Partition1 struct {
	PartStatus      byte     // Indica si la partición está montada o no
	PartType        byte     // Tipo de partición: 'P' (Primaria) o 'E' (Extendida)
	PartFit         byte     // Tipo de ajuste: 'B', 'F', o 'W'
	PartStart       int64    // Byte del disco donde inicia la partición
	PartS           int64    // Tamaño total de la partición en bytes
	PartName        [16]byte // Nombre de la partición
	PartCorrelative int32    // Correlativo de la partición, inicia en 0 hasta que se monte
	PartId          [4]byte  // ID de la partición generado al montar
}
type Partition struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Type string `json:"type"`
	Fit  string `json:"fit"`
}

// -------------------------------- FDISK-DISCOS--------------------------------
func parseFDISKCommand(command2 string) (size int, unit, path, partitionType, fit, deleteOption string, name, add string, err error) {
	//var delete string
	// Dividimos el comando y eliminamos comentarios
	parts := strings.Fields(command2)
	cleanedCommand := strings.SplitN(command2, "#", 2)[0]
	cleanedCommand = strings.TrimSpace(cleanedCommand)
	cleanedCommand = strings.ToLower(cleanedCommand)

	// Valores predeterminados
	unit = "k"          // valor por defecto
	fit = "wf"          // valor por defecto
	partitionType = "p" // valor por defecto

	// Iterar sobre los parámetros para asignarlos correctamente
	for _, part := range parts {
		if strings.HasPrefix(part, "-size=") {
			sizeStr := strings.TrimPrefix(part, "-size=")
			fmt.Sscanf(sizeStr, "%d", &size)
			if size <= 0 {
				err = fmt.Errorf("el tamaño de la partición debe ser mayor a cero")
				return
			}
		} else if strings.HasPrefix(part, "-unit=") {
			unit = strings.TrimPrefix(part, "-unit=")
			if unit != "b" && unit != "k" && unit != "m" {
				err = fmt.Errorf("unidad no válida, debe ser B, K o M")
				return
			}
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
		} else if strings.HasPrefix(part, "-type=") {
			partitionType = strings.TrimPrefix(part, "-type=")
			if partitionType != "p" && partitionType != "e" && partitionType != "l" {
				err = fmt.Errorf("tipo de partición no válido, debe ser P, E o L")
				return
			}
		} else if strings.HasPrefix(part, "-fit=") {
			fit = strings.TrimPrefix(part, "-fit=")
			if fit != "bf" && fit != "ff" && fit != "wf" {
				err = fmt.Errorf("ajuste no válido, debe ser BF, FF o WF")
				return
			}
		} else if strings.HasPrefix(part, "-name=") {
			name = strings.TrimPrefix(part, "-name=")
			// Manejo de comillas alrededor del nombre
			if len(name) > 0 && name[0] == '"' && name[len(name)-1] == '"' {
				name = name[1 : len(name)-1]
			} else {
				re := regexp.MustCompile(`-name="([^"]+)"`)
				matches := re.FindStringSubmatch(cleanedCommand)
				if len(matches) > 1 {
					name = matches[1]
				}
			}
		} else if strings.HasPrefix(part, "-delete=") {
			deleteOption = strings.TrimPrefix(part, "-delete=")
			if deleteOption != "full" && deleteOption != "fast" {
				err = fmt.Errorf("opción de eliminación no válida, debe ser full o fast")
				return
			}
		} else if strings.HasPrefix(part, "-add=") {
			add = strings.TrimPrefix(part, "-add=")
			var addErr error
			if _, addErr = fmt.Sscanf(add, "%d", &size); addErr != nil {
				err = fmt.Errorf("valor de add no es un número válido")
				return
			}
		}
	}

	// Validaciones finales
	if size <= 0 && add == "" {
		err = fmt.Errorf("tamaño de partición no especificado o igual a cero")
		return
	}
	if path == "" {
		err = fmt.Errorf("ruta del disco no especificada, disco no creado")
		return
	}
	if name == "" {
		err = fmt.Errorf("nombre de la partición no especificado")
		return
	}
	return size, unit, path, partitionType, fit, deleteOption, name, add, nil
}

func crearParticion(path string, size int64, name string, particionType string) error {
	// Abrir el archivo del disco
	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		err = fmt.Errorf("Error al abrir el archivo del disco: %v", err)
		return err
	}
	defer file.Close()

	// Leer el MBR existente
	var mbr MBR
	if err := binary.Read(file, binary.LittleEndian, &mbr); err != nil {
		err = fmt.Errorf("Error al leer el MBR: %v", err)
		return err
	}

	// Buscar la primera posición libre en el array de particiones
	found := false
	var startPosition int64 = int64(binary.Size(mbr))
	for i := 0; i < len(mbr.Partitions); i++ {
		if mbr.Partitions[i].PartStatus == 0 {
			for j := 0; j < i; j++ {
				if mbr.Partitions[j].PartStatus != 0 {
					startPosition = mbr.Partitions[j].PartStart + mbr.Partitions[j].PartS
					fmt.Printf("Posición de inicio: %d\n", startPosition)
				}
			}

			// Verificar que hay espacio suficiente para la nueva partición
			if startPosition+size > mbr.MbrTamano {
				err = fmt.Errorf("No hay suficiente espacio en el disco para crear la partición.")
				fmt.Println("Error: No hay suficiente espacio en el disco para crear la partición.")
				return err
			}

			// Crear la partición en la posición libre
			mbr.Partitions[i] = Partition1{
				PartStatus: '0',              // Activar la partición
				PartType:   particionType[0], // Suponiendo que es una partición primaria
				PartFit:    mbr.DskFit,       // Utilizar el fit del MBR
				PartStart:  startPosition,
				PartS:      size,
			}
			copy(mbr.Partitions[i].PartName[:], name)

			found = true
			break
		}
	}

	// Verificar si se encontró un espacio para la partición
	if !found {
		err = fmt.Errorf("No hay espacio disponible para una nueva partición.")
		fmt.Println("Error: No hay espacio disponible para una nueva partición.")
		return err
	}

	// Mover el cursor al inicio del archivo para actualizar el MBR
	if _, err := file.Seek(0, 0); err != nil {
		err = fmt.Errorf("Error al posicionarse al inicio del archivo: %v", err)
		fmt.Println("Error al posicionarse al inicio del archivo:", err)
		return err
	}

	// Escribir el MBR actualizado en el archivo
	if err := binary.Write(file, binary.LittleEndian, &mbr); err != nil {
		err = fmt.Errorf("Error al escribir el MBR actualizado: %v", err)
		fmt.Println("Error al escribir el MBR actualizado:", err)
		return err
	}

	// Ahora que la partición ha sido creada en el archivo, también la agregamos a la estructura de discos en memoria
	for i := range disks {
		if disks[i].Path == path {
			// Crear la partición y agregarla al disco en la estructura en memoria
			newPartition := Partition{
				Name: name,
				Size: size,
				Type: particionType,
			}

			// Agregar la partición al disco en memoria
			disks[i].Partitions = append(disks[i].Partitions, newPartition)

			fmt.Println("Partición creada exitosamente y agregada a la estructura en memoria.")
			break
		}
	}

	return nil
}

func eliminarParticion(path, name, deleteType string) error {
	// Abrir el archivo del disco
	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("Error al abrir el archivo del disco: %v", err)
	}
	defer file.Close()

	// Leer el MBR existente
	var mbr MBR
	if err := binary.Read(file, binary.LittleEndian, &mbr); err != nil {
		return fmt.Errorf("Error al leer el MBR: %v", err)
	}

	// Buscar la partición a eliminar
	partitionIndex := -1
	for i := 0; i < len(mbr.Partitions); i++ {
		partName := strings.Trim(string(mbr.Partitions[i].PartName[:]), "\x00")
		if partName == name {
			partitionIndex = i
			break
		}
	}

	// Si la partición no existe, mostrar error
	if partitionIndex == -1 {
		return fmt.Errorf("Error: La partición '%s' no existe en el disco.", name)
	}

	var confirm string
	fmt.Printf("¿Está seguro que desea eliminar la partición '%s'? (s/n): ", name)
	fmt.Scanln(&confirm)
	if confirm != "s" && confirm != "S" {
		return fmt.Errorf("Eliminación de partición cancelada.")
	}

	// Eliminar partición según el tipo de eliminación
	if deleteType == "fast" {
		// Marcar como vacía la tabla de particiones (solo cambia el estado)
		mbr.Partitions[partitionIndex].PartStatus = 0
	} else if deleteType == "full" {
		// Marcar como vacía y rellenar el espacio con '\0'
		mbr.Partitions[partitionIndex].PartStatus = 0

		// Sobrescribir el espacio de la partición con '\0'
		if _, err := file.Seek(mbr.Partitions[partitionIndex].PartStart, 0); err != nil {
			return fmt.Errorf("Error al posicionarse en la partición: %v", err)
		}
		zeroData := make([]byte, mbr.Partitions[partitionIndex].PartS)
		if _, err := file.Write(zeroData); err != nil {
			return fmt.Errorf("Error al sobrescribir la partición con ceros: %v", err)
		}
	} else {
		return fmt.Errorf("Tipo de eliminación no válido: %s", deleteType)
	}

	// Si es una partición extendida, eliminar también las particiones lógicas
	if mbr.Partitions[partitionIndex].PartType == 'e' {
		err = eliminarParticionesLogicas(file, mbr.Partitions[partitionIndex])
		if err != nil {
			return fmt.Errorf("Error al eliminar particiones lógicas: %v", err)
		}
	}

	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("Error al posicionarse al inicio del archivo: %v", err)
	}

	if err := binary.Write(file, binary.LittleEndian, &mbr); err != nil {
		return fmt.Errorf("Error al escribir el MBR actualizado: %v", err)
	}

	fmt.Printf("Partición '%s' eliminada correctamente.\n", name)
	return nil
}

func eliminarParticionesLogicas(file *os.File, extendida Partition1) error {
	// Aquí puedes implementar la lógica para eliminar particiones lógicas
	// dentro de la partición extendida. Dependiendo de cómo manejes las
	// particiones lógicas, necesitarás recorrerlas y marcarlas como eliminadas
	// o sobrescribirlas con ceros (en caso de eliminación "full").
	// En caso de que no haya particiones lógicas, simplemente retornar sin error.
	fmt.Println("Eliminando particiones lógicas dentro de la partición extendida...")

	// Implementar lógica según cómo estén representadas las particiones lógicas
	return nil
}

//fdisk agregar o eliminar particiones
//subirlo a ec2
