package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func findPartitionAndEBR(mbr MBR, id string) (*Partition1, *EBR, error) {
	normalizedID := strings.TrimSpace(strings.ToLower(id))

	for i, partition := range mbr.Partitions {
		if partition.PartStatus != 0 {
			partitionID := strings.TrimSpace(strings.ToLower(strings.Trim(string(partition.PartId[:]), "\x00")))
			//fmt.Printf("Comparando IDs: Buscado = '%s', Actual = '%s'\n", normalizedID, partitionID)
			//fmt.Printf("  ID: %s\n", strings.Trim(string(partition.PartId[:]), "\x00"))
			if partitionID == normalizedID {
				// Si la partición es extendida, busca EBRs dentro
				if partition.PartType == 'e' {
					ebr := &EBR{}
					return &mbr.Partitions[i], ebr, nil
				}
				return &mbr.Partitions[i], nil, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("no se encontró la partición con ID: %s", id)
}

/*--------------------------------Reportes----------------------------------------------*/
func parseRepCommand(command2 string) (id string, path string, name string, pathfile string, err error) {
	parts := strings.Fields(strings.ToLower(command2))
	cleanedCommand := strings.SplitN(command2, "#", 2)[0]
	cleanedCommand = strings.TrimSpace(cleanedCommand)
	cleanedCommand = strings.ToLower(cleanedCommand)

	for _, part := range parts {

		if strings.HasPrefix(part, "-id=") {
			id = strings.TrimPrefix(part, "-id=")
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
		} else if strings.HasPrefix(part, "-name=") {
			name = strings.TrimPrefix(part, "-name=")
			if len(name) > 0 && name[0] == '"' && name[len(name)-1] == '"' {
				name = name[1 : len(name)-1]
			} else {
				re := regexp.MustCompile(`-name="([^"]+)"`)
				matches := re.FindStringSubmatch(cleanedCommand)
				if len(matches) > 1 {
					name = matches[1]
				}
			}
		} else if strings.HasPrefix(part, "-path_file_ls=") {
			pathfile = strings.TrimPrefix(part, "-path_file_ls=")
			// Manejo de comillas alrededor de la ruta
			if len(pathfile) > 0 && pathfile[0] == '"' && pathfile[len(pathfile)-1] == '"' {
				pathfile = pathfile[1 : len(pathfile)-1]
			} else {
				re := regexp.MustCompile(`-path_file_ls="([^"]+)"`)
				matches := re.FindStringSubmatch(cleanedCommand)
				if len(matches) > 1 {
					pathfile = matches[1]
				}
			}
		}

	}

	return
}

func imprimirMBR_Partitions(path string) {
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

	// Imprimir información del MBR
	fmt.Printf("\nInformación del MBR en el disco '%s':\n", path)
	fmt.Println("------------------------------------------------")
	fmt.Printf("Tamaño del disco: %d bytes\n", mbr.MbrTamano)
	fechaCreacion := string(mbr.MbrFechaCreacion[:])
	fmt.Printf("Fecha de creación: %s\n", fechaCreacion)
	fmt.Printf("Firma del disco: %d\n", mbr.MbrDskSignature)
	fmt.Printf("Ajuste de partición: %c\n", mbr.DskFit)
	fmt.Println("------------------------------------------------")
	// Imprimir información de las particiones
	for i, partition := range mbr.Partitions {
		// Verificar que la partición está activa
		if partition.PartStatus != 0 {
			fmt.Printf("\nPartición %d:\n", i+1)
			fmt.Printf("  Estado: %c\n", partition.PartStatus)
			fmt.Printf("  Tipo: %c\n", partition.PartType)
			fmt.Printf("  Ajuste: %c\n", partition.PartFit)
			fmt.Printf("  Inicio: %d\n", partition.PartStart)
			fmt.Printf("  Tamaño: %d bytes\n", partition.PartS)
			fmt.Printf("  Nombre: %s\n", strings.Trim(string(partition.PartName[:]), "\x00"))
			fmt.Printf("  ID: %s\n", strings.Trim(string(partition.PartId[:]), "\x00"))

			// Si la partición es logica, buscar e imprimir los EBRs
			if partition.PartType == 'l' {
				printEBRs(file, partition.PartStart)
			}
		}
	}
}

func Report_MBR_EBRs(path_disk string, path string) error {
	// Cambiar la extensión del outputPath a .dot
	dotPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".dot"

	// Crear el archivo .dot
	file, err := os.Create(dotPath)
	if err != nil {
		return fmt.Errorf("error al crear el archivo .dot: %v", err)
	}
	defer file.Close()

	// Abrir el archivo del disco
	files, err := os.Open(path_disk)
	if err != nil {
		fmt.Println("Error al abrir el archivo del disco:", err)
		return err
	}
	defer files.Close()

	// Leer el MBR existente
	var mbr MBR
	if err := binary.Read(files, binary.LittleEndian, &mbr); err != nil {
		fmt.Println("Error al leer el MBR:", err)
		return err
	}

	// Escribir el contenido en formato .dot con una tabla
	fmt.Fprintln(file, "digraph G {")
	fmt.Fprintln(file, "    node [shape=plaintext];")
	fmt.Fprintln(file, "    rankdir=LR;")
	fmt.Fprintln(file, "    subgraph cluster_mbr {")
	fmt.Fprintln(file, "        label=\"MBR\";")

	// Escribir la tabla del MBR
	fmt.Fprintln(file, "        mbr_table [label=<")
	fmt.Fprintln(file, "        <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"5\">")
	fmt.Fprintln(file, "            <TR>")
	fmt.Fprintln(file, "                <TD COLSPAN=\"2\" BGCOLOR=\"lightgrey\"><B>MBR</B></TD>")
	fmt.Fprintln(file, "            </TR>")
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Size:</TD><TD ALIGN=\"LEFT\">%d bytes</TD></TR>\n", mbr.MbrTamano)
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">CreatedAt:</TD><TD ALIGN=\"LEFT\">%s</TD></TR>\n", string(mbr.MbrFechaCreacion[:]))
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Signature:</TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n", mbr.MbrDskSignature)
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Fit:</TD><TD ALIGN=\"LEFT\">%c</TD></TR>\n", mbr.DskFit)
	fmt.Fprintln(file, "        </TABLE>")
	fmt.Fprintln(file, "        >];")
	fmt.Fprintln(file, "    }")

	// Escribir todas las particiones en subclusters
	for i, partition := range mbr.Partitions {
		if partition.PartStatus != 0 {
			partitionID := strings.Trim(string(partition.PartId[:]), "\x00")

			fmt.Fprintf(file, "    subgraph cluster_partition%d {\n", i+1)
			fmt.Fprintf(file, "        label=\"Partition %d\";\n", i+1)

			fmt.Fprintf(file, "        partition_table %d [label=<", i+1)
			fmt.Fprintln(file, "        <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"5\">")
			fmt.Fprintln(file, "            <TR>")
			fmt.Fprintf(file, "                <TD COLSPAN=\"2\" BGCOLOR=\"lightgrey\"><B>Partition %d</B></TD>\n", i+1)
			fmt.Fprintln(file, "            </TR>")
			fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Status:</TD><TD ALIGN=\"LEFT\">%c</TD></TR>\n", partition.PartStatus)
			fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Type:</TD><TD ALIGN=\"LEFT\">%c</TD></TR>\n", partition.PartType)
			fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Fit:</TD><TD ALIGN=\"LEFT\">%c</TD></TR>\n", partition.PartFit)
			fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Start:</TD><TD ALIGN=\"LEFT\">%d bytes</TD></TR>\n", partition.PartStart)
			fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Size:</TD><TD ALIGN=\"LEFT\">%d bytes</TD></TR>\n", partition.PartS)
			fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Name:</TD><TD ALIGN=\"LEFT\">%s</TD></TR>\n", strings.Trim(string(partition.PartName[:]), "\x00"))
			fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">ID:</TD><TD ALIGN=\"LEFT\">%s</TD></TR>\n", partitionID)
			fmt.Fprintln(file, "        </TABLE>")
			fmt.Fprintln(file, "        >];")
			fmt.Fprintln(file, "    }")

			// Si la partición es extendida, buscar e imprimir los EBRs
			if partition.PartType == 'e' {
				var ebr EBR
				currentPosition := partition.PartStart
				ebrIndex := 1

				for {
					if _, err := files.Seek(currentPosition, 0); err != nil {
						fmt.Println("Error al posicionarse en el EBR:", err)
						return err
					}
					if err := binary.Read(files, binary.LittleEndian, &ebr); err != nil {
						fmt.Println("Error al leer el EBR:", err)
						return err
					}
					fmt.Printf("EBR encontrado: Inicio=%d bytes, Tamaño=%d bytes, Siguiente=%d bytes\n",
						ebr.Start, ebr.Size, ebr.Next)

					fmt.Fprintf(file, "        subgraph cluster_ebr%d {\n", ebrIndex)
					fmt.Fprintf(file, "            label=\"EBR %d\";\n", ebrIndex)
					fmt.Fprintf(file, "            ebr_table %d[label=<", ebrIndex)
					fmt.Fprintln(file, "            <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"5\">")
					fmt.Fprintln(file, "                <TR>")
					fmt.Fprintln(file, "                    <TD COLSPAN=\"2\" BGCOLOR=\"lightgrey\"><B>EBR</B></TD>")
					fmt.Fprintln(file, "                </TR>")
					fmt.Fprintf(file, "                <TR><TD ALIGN=\"LEFT\">Mount:</TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n", currentPosition)
					fmt.Fprintf(file, "                <TR><TD ALIGN=\"LEFT\">Fit:</TD><TD ALIGN=\"LEFT\">%c</TD></TR>\n", ebr.Fit)
					fmt.Fprintf(file, "                <TR><TD ALIGN=\"LEFT\">Start:</TD><TD ALIGN=\"LEFT\">%d bytes</TD></TR>\n", ebr.Start)
					fmt.Fprintf(file, "                <TR><TD ALIGN=\"LEFT\">Size:</TD><TD ALIGN=\"LEFT\">%d bytes</TD></TR>\n", ebr.Size)
					fmt.Fprintf(file, "                <TR><TD ALIGN=\"LEFT\">Next:</TD><TD ALIGN=\"LEFT\">%d bytes</TD></TR>\n", ebr.Next)
					fmt.Fprintf(file, "                <TR><TD ALIGN=\"LEFT\">Name:</TD><TD ALIGN=\"LEFT\">%s</TD></TR>\n", strings.Trim(string(ebr.Name[:]), "\x00"))
					fmt.Fprintln(file, "            </TABLE>")
					fmt.Fprintln(file, "            >];")
					fmt.Fprintln(file, "        }")

					ebrIndex++
					if ebr.Next == -1 {
						break
					}

					currentPosition = ebr.Next
				}
			}
		}
	}
	fmt.Fprintln(file, "}")

	// Cerrar el archivo .dot y renderizarlo al formato final
	file.Close()
	return renderDotFile(dotPath, path)
}

// Función para renderizar el archivo .dot al formato especificado
func renderDotFile(dotPath, outputPath string) error {
	// Verificar que el archivo .dot existe y se puede leer
	// content, err := os.ReadFile(dotPath)
	// if err != nil {
	// 	return fmt.Errorf("error al leer el archivo .dot: %v", err)
	// }
	//fmt.Println("Contenido del archivo .dot:")
	//fmt.Println(string(content))

	// Comando para renderizar el archivo .dot
	format := strings.TrimPrefix(filepath.Ext(outputPath), ".")
	cmd := exec.Command("dot", "-T"+format, "-o", outputPath, dotPath)
	fmt.Printf("dot -T%s -o %s %s\n", format, outputPath, dotPath)
	//fmt.Printf("Ejecutando comando: sudo dot -T%s -o %s %s\n", format, outputPath, dotPath)

	// Capturar errores del comando
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error al renderizar el archivo .dot: %v, salida: %s", err, string(output))
	}
	return nil
}

func generateDiskReport(mbr MBR, outputPath string, diskName string) error {
	// Cambiar la extensión del outputPath a .dot
	dotPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".dot"

	// Crear el archivo .dot
	file, err := os.Create(dotPath)
	if err != nil {
		return fmt.Errorf("error al crear el archivo .dot: %v", err)
	}
	defer file.Close()

	// Escribir el contenido en formato .dot
	fmt.Fprintln(file, "digraph G {")
	fmt.Fprintln(file, "  node [shape=box];")
	fmt.Fprintln(file, "  rankdir=LR;")
	fmt.Fprintf(file, "  label=\"%s\";\n", diskName)

	usedSpace := int64(0)
	for _, partition := range mbr.Partitions {
		if partition.PartStatus != 0 {
			usedSpace += partition.PartS
		}
	}

	freeSpace := mbr.Partitions[0].PartStart
	fmt.Fprintf(file, "  mbr [label=\"MBR\\n%.2f%% del disco\", shape=box];\n", (float64(freeSpace)/float64(mbr.MbrTamano))*100)

	for i, partition := range mbr.Partitions {
		if partition.PartStatus == 0 {
			continue
		}

		partitionName := strings.Trim(string(partition.PartName[:]), "\x00")
		percentage := (float64(partition.PartS) / float64(mbr.MbrTamano)) * 100

		if partition.PartType == 'p' {
			fmt.Fprintf(file, "  primary%d [label=\"%s\\n%.2f%% del disco\", shape=box];\n", i, partitionName, percentage)
		} else if partition.PartType == 'e' {
			fmt.Fprintf(file, "  subgraph cluster_extended%d {\n", i)
			fmt.Fprintf(file, "    label=\"Extendida\";\n")
			// Aquí se leerán los EBRs si es necesario
			fmt.Fprintln(file, "  }")
		}
	}

	fmt.Fprintln(file, "}")

	// Cerrar el archivo .dot y renderizarlo al formato final
	file.Close()
	return renderDotFile(dotPath, outputPath)
}

/*---------------------------Reporte SB---------------------------------*/

func imprimirSuperBloque(file *os.File, start int64) {
	var superblock SuperBlock
	if _, err := file.Seek(start, 0); err != nil {
		fmt.Println("Error al posicionarse en el inicio del super bloque:", err)
		return
	}
	if err := binary.Read(file, binary.LittleEndian, &superblock); err != nil {
		fmt.Println("Error al leer el super bloque:", err)
		return
	}

	fmt.Println("Datos del Super Bloque:")
	fmt.Printf("Filesystem Type: %d\n", superblock.FilesystemType)
	fmt.Printf("Inodes Count: %d\n", superblock.InodesCount)
	fmt.Printf("Blocks Count: %d\n", superblock.BlocksCount)
	fmt.Printf("Free Blocks Count: %d\n", superblock.FreeBlocksCount)
	fmt.Printf("Free Inodes Count: %d\n", superblock.FreeInodesCount)
	fmt.Printf("Mount Time: %s\n", superblock.MountTime)
	fmt.Printf("Unmount Time: %s\n", superblock.UnmountTime)
	fmt.Printf("Mount Count: %d\n", superblock.MountCount)
	fmt.Printf("Magic: 0x%X\n", superblock.Magic)
	fmt.Printf("Inode Size: %d\n", superblock.InodeSize)
	fmt.Printf("Block Size: %d\n", superblock.BlockSize)
	fmt.Printf("First Inode: %d\n", superblock.FirstInode)
	fmt.Printf("First Block: %d\n", superblock.FirstBlock)
	fmt.Printf("Bitmap Inode Start: %d\n", superblock.BmInodeStart)
	fmt.Printf("Bitmap Block Start: %d\n", superblock.BmBlockStart)
	fmt.Printf("Inode Start: %d\n", superblock.InodeStart)
	fmt.Printf("Block Start: %d\n", superblock.BlockStart)
}

func LeerSuperBloquePorID(id string) (*SuperBlock, error) {
	// Verificar si la partición está montada usando el ID
	partition, exists := mountedPartitions[id]
	if !exists {
		return nil, fmt.Errorf("partición con ID '%s' no está montada", id)
	}

	// Abrir el archivo del disco en modo de lectura
	file, err := os.Open(partition.Path)
	if err != nil {
		return nil, fmt.Errorf("error al abrir el archivo: %v", err)
	}
	defer file.Close()

	// Determinar la posición del SuperBlock basado en la ubicación de la partición
	offset := partition.Partition.PartStart // Ajusta esta línea si la posición del super bloque es diferente
	//fmt.Println("Posicionándose en el offset:", offset)

	// Posicionarse en el offset correcto de la partición
	if _, err := file.Seek(offset, 0); err != nil {
		return nil, fmt.Errorf("error al posicionarse en el offset %d: %v", offset, err)
	}

	// Leer el superbloque desde el archivo
	var superblock SuperBlock
	if err := binary.Read(file, binary.LittleEndian, &superblock); err != nil {
		return nil, fmt.Errorf("error al leer el superbloque: %v", err)
	}

	// Retornar el superbloque leído
	return &superblock, nil
}

func Report_SuperBlock(superblock SuperBlock, outputPath string) error {
	// Cambiar la extensión del outputPath a .dot
	dotPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".dot"
	//fmt.Println(dotPath)
	//fmt.Println(outputPath)
	// Crear el archivo .dot
	file, err := os.Create(dotPath)
	if err != nil {
		return fmt.Errorf("error al crear el archivo .dot: %v", err)
	}
	defer file.Close()

	fmt.Fprintln(file, "digraph G {")
	fmt.Fprintln(file, "    node [shape=plaintext];")
	fmt.Fprintln(file, "    rankdir=TB;")
	fmt.Fprintln(file, "    subgraph cluster_superblock {")
	fmt.Fprintln(file, "        label=\"Super Block\";")

	fmt.Fprintln(file, "        superblock_table [label=<")
	fmt.Fprintln(file, "        <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"5\">")
	fmt.Fprintln(file, "            <TR>")
	fmt.Fprintln(file, "                <TD COLSPAN=\"2\" BGCOLOR=\"lightgrey\"><B>Super Block</B></TD>")
	fmt.Fprintln(file, "            </TR>")
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Filesystem Type:</TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n", superblock.FilesystemType)
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Inodes Count:</TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n", superblock.InodesCount)
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Blocks Count:</TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n", superblock.BlocksCount)
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Free Blocks Count:</TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n", superblock.FreeBlocksCount)
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Free Inodes Count:</TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n", superblock.FreeInodesCount)
	mountTimeStr := string(superblock.MountTime[:])
	mountTime, _ := time.Parse("2006-01-02 15:04:05", mountTimeStr)
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Mount Time:</TD><TD ALIGN=\"LEFT\">%s</TD></TR>\n", mountTime.Format("2006-01-02 15:04:05"))
	unmountTimeStr := string(superblock.UnmountTime[:])
	unmountTime, _ := time.Parse("2006-01-02 15:04:05", unmountTimeStr)
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Unmount Time:</TD><TD ALIGN=\"LEFT\">%s</TD></TR>\n", unmountTime.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Mount Count:</TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n", superblock.MountCount)
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Magic:</TD><TD ALIGN=\"LEFT\">0x%X</TD></TR>\n", superblock.Magic)
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Inode Size:</TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n", superblock.InodeSize)
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Block Size:</TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n", superblock.BlockSize)
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">First Inode:</TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n", superblock.FirstInode)
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">First Block:</TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n", superblock.FirstBlock)
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Bitmap Inode Start:</TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n", superblock.BmInodeStart)
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Bitmap Block Start:</TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n", superblock.BmBlockStart)
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Inode Start:</TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n", superblock.InodeStart)
	fmt.Fprintf(file, "            <TR><TD ALIGN=\"LEFT\">Block Start:</TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n", superblock.BlockStart)
	fmt.Fprintln(file, "        </TABLE>")
	fmt.Fprintln(file, "        >];")

	fmt.Fprintln(file, "    }")
	fmt.Fprintln(file, "}")

	// Cerrar el archivo .dot y renderizarlo al formato final si es necesario
	file.Close()
	return renderDotFile(dotPath, outputPath)
}
