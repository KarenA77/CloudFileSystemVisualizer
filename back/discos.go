package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

type EBR struct {
	Mount byte     // particion montada, no montada
	Fit   byte     // ajuste B, F, W
	Start int64    // Byte de inicio del disco
	Size  int64    // tamaño de la particion bytes
	Next  int64    // Byte del proximo EBR
	Name  [16]byte // nombre particion
}

type MountedPartition struct {
	ID        string // ID generado para la partición montada
	Path      string // Ruta del disco
	Partition Partition1
}

type Usuario struct {
	User  string
	Pass  string
	Grupo string
}

// Analiza una cadena de comando y devuelve los parámetros extraídos.
func parseCommand(command string) (size int, unit, path, partitionType, fit, name, delete string, add, id, fsType string, full bool, username, password string, pathfile string, log, err error) {
	command = strings.TrimSpace(command)
	var command2 string = strings.ToLower(command)

	//fmt.Println("Comando parse:", command)

	// Administración de discos
	if strings.HasPrefix(command2, "mkdisk") {
		var size64 int64
		size64, unit, path, fit, err = parseMkdirCommand(command2)
		//fmt.Println("Size:", size64, "Unit:", unit, "Path:", path, "Fit:", fit)
		if err != nil {
			fmt.Println("Error al analizar mkdisk:", err)
			return
		}
		size = int(size64)
	} else if strings.HasPrefix(command2, "rmdisk") {
		path, err = parseRmDiskCommand(command2)
		if err != nil {
			fmt.Println("Error al analizar rmdisk:", err)
			return
		}
		//fmt.Println("Path:", path)
	} else if strings.HasPrefix(command2, "fdisk") {
		size, unit, path, partitionType, fit, delete, name, add, err = parseFDISKCommand(command2)
	} else if strings.HasPrefix(command2, "mount") {
		path, name, err = parseMountCommand(command)

		//Administración del Sistema de Archivos
	} else if strings.HasPrefix(command, "login") {
		username, password, id, err = parseLoginCommand(command)
	} else if strings.HasPrefix(command, "logout") {
		_, err = parseLogoutCommand(command)
	} else if strings.HasPrefix(command2, "mkfs") {
		id, fsType, full, err = parseMkfsCommand(command2)
	} else if strings.HasPrefix(command, "cat") {
		//id, fsType, full, err = parseMkfsCommand(command)
	} else if strings.HasPrefix(command, "mkgrp") {
		//id, path, name = parseRepCommand(command)
	} else if strings.HasPrefix(command, "rmgrp") {
		//id, path, name = parseRepCommand(command)
	} else if strings.HasPrefix(command, "mkusr") {
		//id, path, name = parseRepCommand(command)
	} else if strings.HasPrefix(command, "rmusr") {
		//id, path, name = parseRepCommand(command)
	} else if strings.HasPrefix(command, "chgrp") {
		//id, path, name = parseRepCommand(command)
	} else if strings.HasPrefix(command, "mkfile") {
		//id, path, name = parseRepCommand(command)
	} else if strings.HasPrefix(command, "mkdir") {
		//id, path, name = parseRepCommand(command)
	} else if strings.HasPrefix(command2, "rep") {
		id, path, name, pathfile, err = parseRepCommand(command2)
	} else {
		err = fmt.Errorf("comando no reconocido")
	}
	return
}

func crearParticionLogica(path string, size int64, name string, fit string) error {
	// Abrir el archivo del disco
	file, err := os.OpenFile(path, os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("Error al abrir el disco: %v", err)
	}
	defer file.Close()

	// Leer el MBR
	var mbr MBR
	if err := binary.Read(file, binary.LittleEndian, &mbr); err != nil {
		return fmt.Errorf("Error al leer el MBR: %v", err)
	}

	// Buscar la partición extendida
	var extendedPartition Partition1
	foundExtended := false
	for _, partition := range mbr.Partitions {
		if partition.PartType == 'e' || partition.PartType == 'E' {
			extendedPartition = partition
			foundExtended = true
			break
		}
	}

	if !foundExtended {
		return fmt.Errorf("No existe una partición extendida en el disco")
	}

	// Posicionarse al inicio de la partición extendida
	currentEBRPosition := extendedPartition.PartStart
	var prevEBR *EBR = nil

	// Recorrer los EBR existentes
	for {
		// Leer el EBR en la posición actual
		var ebr EBR
		if _, err := file.Seek(currentEBRPosition, 0); err != nil {
			return fmt.Errorf("Error al posicionarse en el EBR: %v", err)
		}
		if err := binary.Read(file, binary.LittleEndian, &ebr); err != nil {
			return fmt.Errorf("Error al leer el EBR: %v", err)
		}

		if ebr.Size == 0 {
			// No hay más particiones lógicas, crear aquí
			break
		}

		// Verificar si ya existe una partición lógica con el mismo nombre
		if strings.Trim(string(ebr.Name[:]), "\x00") == name {
			return fmt.Errorf("Ya existe una partición lógica con el nombre '%s'", name)
		}

		// Continuar al siguiente EBR
		if ebr.Next == -1 || ebr.Next == 0 {
			prevEBR = &ebr
			break
		} else {
			currentEBRPosition = ebr.Next
			prevEBR = &ebr
		}
	}

	// Calcular la posición de inicio para la nueva partición lógica
	var newEBRStart int64
	if prevEBR == nil {
		// Primera partición lógica
		newEBRStart = extendedPartition.PartStart
	} else {
		newEBRStart = prevEBR.Start + prevEBR.Size
	}

	// Verificar que haya suficiente espacio
	if newEBRStart+size > extendedPartition.PartStart+extendedPartition.PartS {
		return fmt.Errorf("No hay suficiente espacio para la nueva partición lógica")
	}

	// Crear el nuevo EBR
	newEBR := EBR{
		Mount: '0',
		Fit:   fit[0],
		Start: newEBRStart,
		Size:  size,
		Next:  -1,
	}
	copy(newEBR.Name[:], name)

	// Escribir el nuevo EBR
	if _, err := file.Seek(newEBRStart, 0); err != nil {
		return fmt.Errorf("Error al posicionarse para escribir el nuevo EBR: %v", err)
	}
	if err := binary.Write(file, binary.LittleEndian, &newEBR); err != nil {
		return fmt.Errorf("Error al escribir el nuevo EBR: %v", err)
	}

	// Actualizar el EBR anterior, si existe
	if prevEBR != nil {
		prevEBR.Next = newEBRStart
		if _, err := file.Seek(prevEBR.Start, 0); err != nil {
			return fmt.Errorf("Error al posicionarse para actualizar el EBR anterior: %v", err)
		}
		if err := binary.Write(file, binary.LittleEndian, prevEBR); err != nil {
			return fmt.Errorf("Error al actualizar el EBR anterior: %v", err)
		}
	} else {
		// Si es el primer EBR, asegurarse de escribirlo al inicio de la partición extendida
		if _, err := file.Seek(extendedPartition.PartStart, 0); err != nil {
			return fmt.Errorf("Error al posicionarse para escribir el primer EBR: %v", err)
		}
		if err := binary.Write(file, binary.LittleEndian, &newEBR); err != nil {
			return fmt.Errorf("Error al escribir el primer EBR: %v", err)
		}
	}
	//printEBRs(file, extendedPartition.PartStart)
	//fmt.Printf("Partición lógica '%s' creada con éxito.\n", name)
	return nil
}

// Función para imprimir particiones y EBRs
func imprimirPartitions(file *os.File, mbr *MBR) {
	fmt.Println("\nParticiones en el disco:")
	for i, partition := range mbr.Partitions {
		if partition.PartStatus != 0 {
			fmt.Printf("\nPartición %d:\n", i+1)
			fmt.Printf("  Tipo: %c\n", partition.PartType)
			fmt.Printf("  Ajuste: %c\n", partition.PartFit)
			fmt.Printf("  Inicio: %d\n", partition.PartStart)
			fmt.Printf("  Tamaño: %d bytes\n", partition.PartS)
			fmt.Printf("  Nombre: %s\n", string(partition.PartName[:]))

			// Si es extendida, imprimir EBRs
			if partition.PartType == 'e' {
				fmt.Println("  EBRs dentro de la partición extendida:")
				var ebr EBR
				currentPosition := partition.PartStart
				for {
					if _, err := file.Seek(currentPosition, 0); err != nil {
						err = fmt.Errorf("Error al posicionarse en el EBR: %v", err)
						return
					}
					if err := binary.Read(file, binary.LittleEndian, &ebr); err != nil {
						err = fmt.Errorf("Error al leer el EBR: %v", err)
						return
					}

					// Imprimir información del EBR
					fmt.Printf("\nEBR en posición: %d\n", currentPosition)
					fmt.Printf("  Montado: %c\n", ebr.Mount)
					fmt.Printf("  Ajuste: %c\n", ebr.Fit)
					fmt.Printf("  Inicio: %d\n", ebr.Start)
					fmt.Printf("  Tamaño: %d bytes\n", ebr.Size)
					fmt.Printf("  Siguiente EBR: %d\n", ebr.Next)
					fmt.Printf("  Nombre: %s\n", strings.Trim(string(ebr.Name[:]), "\x00"))

					// Si no hay un siguiente EBR, salir del bucle
					if ebr.Next == 0 {
						break
					}

					// Mover a la posición del siguiente EBR
					currentPosition = ebr.Next

				}
			}
		}
	}
}

/*
if partition.PartType == 'e' {
				fmt.Println("  EBRs dentro de la partición extendida:")
				ebrPos := partition.PartStart
				for {


					ebr, err := readEBR(file, ebrPos)
					if err != nil {
						fmt.Println("    Error al leer el EBR:", err)
						break
					}
					fmt.Printf("    EBR en %d: Inicio=%d, Tamaño=%d, Siguiente=%d, Nombre=%s\n",
						ebrPos, ebr.Start, ebr.Size, ebr.Next, string(ebr.Name[:]))
					if ebr.Next == 0 {
						break
					}
					ebrPos = ebr.Next
				}
			}
*/

// -----------------------------------CREAR EBR-----------------------------------------
// Función para crear un nuevo EBR dentro de una partición extendida
func crearEBR(file *os.File, start int64, size int64, name string) error {
	// Crear un nuevo EBR con los valores iniciales
	ebr := EBR{
		Mount: 0,     // No montada
		Fit:   'w',   // Ajuste por defecto
		Start: start, // Inicio del EBR
		Size:  size,  // Tamaño de la partición lógica
		Next:  0,     // Inicialmente, no hay un siguiente EBR
	}
	copy(ebr.Name[:], name) // Copiar el nombre de la partición en el campo Name del EBR

	// Verificar si hay un EBR anterior que deba actualizarse
	lastEBR, err := findLastEBR(file, start)
	if err == nil && lastEBR.Next == 0 {
		// Actualizar el campo Next del EBR anterior para apuntar al nuevo EBR
		lastEBR.Next = start
		if err := writeEBR(file, lastEBR, lastEBR.Start); err != nil {
			fmt.Println("Error al actualizar el EBR anterior:", err)
			return err
		}
	}

	// Escribir el nuevo EBR en el archivo de disco
	if err := writeEBR(file, &ebr, start); err != nil {
		fmt.Println("Error al escribir el nuevo EBR:", err)
		return err
	}

	fmt.Println("EBR creado exitosamente en la posición:", start)
	return nil
}

// Función auxiliar para encontrar el último EBR en una cadena de EBRs
func findLastEBR(file *os.File, start int64) (*EBR, error) {
	currentStart := start

	// Leer EBRs hasta encontrar el último, es decir, cuando Next sea 0
	for {
		ebr, err := readEBR(file, currentStart)
		if err != nil {
			return nil, fmt.Errorf("error al leer el EBR: %v", err)
		}

		// Si Next es 0, este es el último EBR
		if ebr.Next == 0 {
			return ebr, nil
		}

		// Continuar con el siguiente EBR
		currentStart = ebr.Next
	}
}

// Función para escribir un EBR en una posición específica del archivo de disco
func writeEBR(file *os.File, ebr *EBR, start int64) error {
	var buffer bytes.Buffer

	// Escribir los campos del EBR en el buffer
	if err := binary.Write(&buffer, binary.LittleEndian, ebr.Mount); err != nil {
		return fmt.Errorf("error al escribir Mount: %v", err)
	}
	if err := binary.Write(&buffer, binary.LittleEndian, ebr.Fit); err != nil {
		return fmt.Errorf("error al escribir Fit: %v", err)
	}
	if err := binary.Write(&buffer, binary.LittleEndian, ebr.Start); err != nil {
		return fmt.Errorf("error al escribir Start: %v", err)
	}
	if err := binary.Write(&buffer, binary.LittleEndian, ebr.Size); err != nil {
		return fmt.Errorf("error al escribir Size: %v", err)
	}
	if err := binary.Write(&buffer, binary.LittleEndian, ebr.Next); err != nil {
		return fmt.Errorf("error al escribir Next: %v", err)
	}
	if _, err := buffer.Write(ebr.Name[:]); err != nil {
		return fmt.Errorf("error al escribir Name: %v", err)
	}

	// Escribir el buffer en el archivo en la posición especificada
	if _, err := file.WriteAt(buffer.Bytes(), start); err != nil {
		return fmt.Errorf("error al escribir EBR en el archivo: %v", err)
	}

	return nil
}

// Función para leer un EBR desde un archivo en una posición específica
func readEBR(file *os.File, start int64) (*EBR, error) {
	// Mover el cursor a la posición de inicio
	if _, err := file.Seek(start, 0); err != nil {
		return nil, fmt.Errorf("error al posicionarse en el archivo: %v", err)
	}

	var ebr EBR
	// Leer los datos del EBR
	if err := binary.Read(file, binary.LittleEndian, &ebr); err != nil {
		return nil, fmt.Errorf("error al leer el EBR: %v", err)
	}

	return &ebr, nil
}

// Función para leer todos los EBRs en una partición extendida dado el path y el inicio
func readAllEBRs(path string, start int64) ([]EBR, error) {
	var ebrs []EBR
	currentStart := start

	// Abrir el archivo en modo de lectura
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error al abrir el archivo: %v", err)
	}
	defer file.Close()

	fmt.Printf("Leyendo EBRs desde la partición extendida en el disco: %s\n", path)

	// Leer todos los EBRs hasta que el campo Next sea 0 (fin de la lista)
	for {
		var ebr EBR
		if _, err := file.Seek(currentStart, 0); err != nil {
			return nil, fmt.Errorf("error al posicionarse en el archivo: %v", err)
		}

		if err := binary.Read(file, binary.LittleEndian, &ebr); err != nil {
			return nil, fmt.Errorf("error al leer el EBR: %v", err)
		}

		// Imprimir los detalles del EBR leído
		fmt.Printf("EBR en posición: %d\n", currentStart)
		fmt.Printf("  Montado: %c\n", ebr.Mount)
		fmt.Printf("  Ajuste: %c\n", ebr.Fit)
		fmt.Printf("  Inicio: %d bytes\n", ebr.Start)
		fmt.Printf("  Tamaño: %d bytes\n", ebr.Size)
		fmt.Printf("  Siguiente EBR: %d\n", ebr.Next)
		fmt.Printf("  Nombre: %s\n", strings.Trim(string(ebr.Name[:]), "\x00"))

		ebrs = append(ebrs, ebr)

		// Si Next es 0, se ha llegado al final de los EBRs
		if ebr.Next == 0 {
			break
		}
		currentStart = ebr.Next
	}

	return ebrs, nil
}

// Función para imprimir todos los EBRs dentro de una partición extendida
func printEBRs(file *os.File, start int64) {
	var ebr EBR
	currentPosition := start

	fmt.Println("\nEBRs en la partición extendida:")

	for {
		// Leer el EBR en la posición actual
		if _, err := file.Seek(currentPosition, 0); err != nil {
			fmt.Println("Error al posicionarse en el EBR:", err)
			return
		}
		if err := binary.Read(file, binary.LittleEndian, &ebr); err != nil {
			fmt.Println("Error al leer el EBR:", err)
			return
		}

		// Imprimir información del EBR
		fmt.Printf("\nEBR en posición: %d\n", currentPosition)
		fmt.Printf("  Montado: %c\n", ebr.Mount)
		fmt.Printf("  Ajuste: %c\n", ebr.Fit)
		fmt.Printf("  Inicio: %d\n", ebr.Start)
		fmt.Printf("  Tamaño: %d bytes\n", ebr.Size)
		fmt.Printf("  Siguiente EBR: %d\n", ebr.Next)
		fmt.Printf("  Nombre: %s\n", strings.Trim(string(ebr.Name[:]), "\x00"))

		// Si no hay un siguiente EBR, salir del bucle
		if ebr.Next == 0 {
			break
		}

		// Mover a la posición del siguiente EBR
		currentPosition = ebr.Next
	}
}

// ------------------------------Montar particion -------------------------------
func parseMountCommand(command2 string) (path, name string, err error) {
	//fmt.Println("Comando mount:", command2)
	command2 = strings.ToLower(command2)

	if !strings.HasPrefix(command2, "mount") {
		return "", "", fmt.Errorf("comando no válido")
	}

	params := strings.Split(command2[len("mount "):], " ")
	for _, param := range params {
		if strings.HasPrefix(param, "-path=") {
			path = strings.TrimPrefix(param, "-path=")
			path = strings.Trim(path, "\"")
		} else if strings.HasPrefix(param, "-name=") {
			name = strings.TrimPrefix(param, "-name=")
			name = strings.Trim(name, "\"")
		}
	}

	if path == "" || name == "" {
		return "", "", fmt.Errorf("los parámetros -path y -name son obligatorios")
	}

	return path, name, nil
}

var mountedPartitions = make(map[string]MountedPartition) // Mapa de particiones montadas
var nextIDNumber = 1                                      // Número inicial de la partición
var nextIDChar = 'A'

func generatePartitionID(carnet string, path string) string {
	// Obtener los últimos dos dígitos del carnet
	lastTwoDigits := "03"

	// Buscar si existe una partición montada del mismo disco
	var existingPartition MountedPartition
	for _, partition := range mountedPartitions {
		if partition.Path == path {
			existingPartition = partition
			break
		}
	}

	// Si hay una partición del mismo disco, incrementar el número de partición
	if existingPartition.ID != "" {
		nextIDNumber++
	} else {
		// Si es de un disco diferente, reiniciar el número y avanzar la letra
		nextIDNumber = 1
		nextIDChar++
		// Reiniciar a 'A' si se ha pasado de 'Z'
		if nextIDChar > 'Z' {
			nextIDChar = 'A'
		}
	}

	// Generar el ID en el formato requerido
	partitionID := fmt.Sprintf("%s%d%c", lastTwoDigits, nextIDNumber, nextIDChar)
	return partitionID
}

// Función para montar una partición primaria
func mountPartition(path, name, carnet string) error {
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

	// Buscar la partición por nombre dentro del MBR
	var partition *Partition1
	found := false
	var partitionIndex int
	for i := 0; i < len(mbr.Partitions); i++ {
		part := &mbr.Partitions[i]
		if strings.Trim(string(part.PartName[:]), "\x00") == name && part.PartType == 'p' {
			partition = part
			partitionIndex = i
			found = true
			break
		}
	}

	// Si no se encuentra la partición, mostrar un error
	if !found {
		return fmt.Errorf("partición '%s' no encontrada en el disco '%s'", name, path)
	}

	// Generar el ID único para la partición
	partitionID := strings.ToLower(generatePartitionID(carnet, path))

	// Actualizar el estado y el correlativo de la partición
	partition.PartStatus = '1'                      // Marcar la partición como montada
	partition.PartCorrelative = int32(nextIDNumber) // Asignar el número correlativo
	copy(partition.PartId[:], partitionID[:4])      // Asignar el ID generado a la partición

	// Verificar si la partición ya está montada
	if _, exists := mountedPartitions[partitionID]; exists {
		fmt.Println("La partición con ID", partitionID, "ya está montada")
		return fmt.Errorf("La partición con ID '%s' ya está montada", partitionID)
	}

	// Escribir los cambios de la partición de vuelta al MBR en el disco
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("Error al posicionarse en el inicio del archivo: %v", err)
	}
	mbr.Partitions[partitionIndex] = *partition
	if err := binary.Write(file, binary.LittleEndian, &mbr); err != nil {
		return fmt.Errorf("Error al escribir los cambios en el MBR: %v", err)
	}

	// Agregar la partición al mapa de particiones montadas en memoria
	mountedPartitions[partitionID] = MountedPartition{
		ID:        partitionID,
		Path:      path,
		Partition: *partition,
	}

	fmt.Printf("Partición '%s' montada con ID '%s'.\n", name, partitionID)
	return nil
}

func isPartitionMounted(path, name string) (bool, MountedPartition) {
	for _, partition := range mountedPartitions {
		// Compara tanto la ruta del disco como el nombre de la partición
		if partition.Path == path && strings.Trim(string(partition.Partition.PartName[:]), "\x00") == name {
			return true, partition
		}
	}
	return false, MountedPartition{}
}
