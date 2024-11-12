package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
)

type Response struct {
	Message     []string    `json:"messages"`
	DiskResoult []Disk      `json:"disk_resoult"`
	PartResoult []Partition `json:"part_resoult"`
	Error       string      `json:"error,omitempty"`
}

var disks []Disk
var mutex sync.Mutex
var partitions []Partition

func messageHandler(w http.ResponseWriter, r *http.Request) {
	response := Response{Message: []string{"Hola desde el backend!"}}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func executeHandler(w http.ResponseWriter, r *http.Request) {
	// Permitir solicitudes CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Manejar solicitudes OPTIONS (preflight)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Manejar solicitudes POST
	fmt.Println("Se ha recibido una solicitud en /execute")
	if r.Method == http.MethodPost {
		var input []string // Cambiamos a slice para manejar múltiples comandos en orden
		response := struct {
			Message     []string    `json:"messages"`
			DiskResoult []Disk      `json:"disk_resoult"`
			PartResoult []Partition `json:"part_resoult"`
			Error       string      `json:"error,omitempty"`
		}{}
		var primaryCount int = 0
		var extendedCount int = 0

		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			response.Error = "Invalid request payload"
			http.Error(w, response.Error, http.StatusBadRequest)
			return
		}

		// Convertir cada comando a minúsculas para evitar problemas de reconocimiento
		for i, cmd := range input {
			input[i] = cmd
		}

		//fmt.Println("Comandos recibidos:", input)
		var isLoggedIn = false
		// Procesar cada comando uno por uno
		for _, cmd := range input {
			trimmedCmd := strings.TrimSpace(cmd)
			if strings.HasPrefix(trimmedCmd, "#") {
				// Imprimir el comentario en la consola del frontend
				fmt.Println(trimmedCmd) // Aquí muestra los comentarios
				response.Message = append(response.Message, trimmedCmd)
				continue // Continuar al siguiente comando
			}
			//fmt.Println("Procesando comando:", cmd)
			size, unit, path, partitionType, fit, name, id, _, _, user, pass, _, _, _, _, err := parseCommand(cmd)
			if err != nil {
				response.Error = fmt.Sprintf("Error al procesar el comando: %s", err)
				response.Message = append(response.Message, fmt.Sprintf("Error: %s", err.Error()))
				//fmt.Println("Error:", err)
				continue
			}

			// Comprobar si el comando es para crear un disco
			cmd = strings.ToLower(cmd)
			if strings.HasPrefix(cmd, "mkdisk") {
				disk, err := processMkdirCommand(cmd)
				if err != nil {
					response.Message = append(response.Message, fmt.Sprintf("Error: %s", err.Error()))
					continue
				}
				// Agregar el mensaje de éxito a la respuesta
				response.Message = append(response.Message, fmt.Sprintf("Disco creado: Size=%d, Unit=%s, Path=%s, Fit=%s", size, unit, path, fit))

				//MANDAR FRONTEND Discos
				mutex.Lock()
				disks = append(disks, disk)
				mutex.Unlock()
				fmt.Println("Disco creado:", disk.Path)

				// Agregar el nuevo disco a la lista `response.DiskResoult`
				response.DiskResoult = append(response.DiskResoult, disk)
				//NO AGREGAR ESTA PARTE AUN
			} else if strings.HasPrefix(cmd, "rmdisk") {
				// Analizar el comando rmdisk y obtener la ruta
				path, err := parseRmDiskCommand(cmd)
				if err != nil {
					response.Message = append(response.Message, fmt.Sprintf("Error: %s", err.Error()))
					continue
				}

				// Encontrar el disco en la lista interna 'disks'
				found := false
				mutex.Lock()
				for i, _ := range disks {
					// Eliminar el archivo del disco del sistema
					err := deleteDisk(path)
					if err != nil {
						response.Message = append(response.Message, fmt.Sprintf("Error al eliminar el disco: %s", err.Error()))
					} else {
						// Eliminar el disco de la lista 'disks'
						disks = append(disks[:i], disks[i+1:]...)
						found = true
						response.Message = append(response.Message, fmt.Sprintf("Disco eliminado: Path=%s", path))
					}
					break
				}
				mutex.Unlock()

				if !found {
					response.Message = append(response.Message, fmt.Sprintf("Error: No se encontró un disco con la ruta %s", path))
				}

				// Actualizar la lista de discos en la respuesta
				response.DiskResoult = disks
			} else if strings.HasPrefix(cmd, "fdisk") {
				var delete string = ""
				var add string = ""
				size, unit, path, partitionType, fit, delete, name, _, err = parseFDISKCommand(cmd)
				if err != nil {
					response.Message = append(response.Message, fmt.Sprintf("Error: %s", err.Error()))
					continue
				}
				//var delete string = ""
				if delete != "" {
					// Aquí manejamos el caso de eliminar la partición
					err := eliminarParticion(path, name, delete)
					if err != nil {
						response.Message = append(response.Message, fmt.Sprintf("Error al eliminar la partición: %s", err.Error()))
						continue
					}
					response.Message = append(response.Message, fmt.Sprintf("Partición eliminada: Path=%s, Name=%s, Método de eliminación=%s", path, name, delete))
				} else if add != "" {
					response.Message = append(response.Message, "Se agregaron: ", add)

				} else {
					if partitionType == "p" {
						if primaryCount >= 4 {
							response.Message = append(response.Message, "Error: No se pueden crear más de 4 particiones primarias.")
							//fmt.Println("Error: No se pueden crear más de 4 particiones primarias.")
							primaryCount = 0
							continue
						}
						//fmt.Println("Unidad de la particion: ", unit)
						//fmt.Println("tamaño particion: ", size)

						// Intentar crear la partición primaria
						var size1 int64
						if unit == "k" { // convertir a bytes
							size1 = int64(size) * 1024
						} else if unit == "m" { // convertir a bytes
							size1 = int64(size) * 1024 * 1024
						} else if unit == "b" {
							size1 = int64(size) * 1
						}

						partition := Partition1{
							PartStatus: '0',              // Marcar la partición como activa
							PartType:   partitionType[0], // Tipo de partición ('p' para primaria, 'e' para extendida)
							PartFit:    fit[0],           // Ajuste de la partición
							PartS:      int64(size),
						}
						copy(partition.PartName[:], name)

						//fmt.Println("tamaño particion2: ", size1)
						err := crearParticion(path, int64(size1), name, partitionType)
						if err != nil {
							response.Message = append(response.Message, fmt.Sprintf("Error al crear la partición: %s", err.Error()))
							//fmt.Println("Error al crear partición:", err)
							continue
						}

						// Incrementar primaryCount solo si no hubo errores
						primaryCount++
						response.Message = append(response.Message, fmt.Sprintf("Partición creada: Size=%d, Unit=%s, Path=%s, Type=%s, Fit=%s, Name=%s", size, unit, path, partitionType, fit, name))

						//fmt.Printf("Partición creada: Size=%d, Unit=%s, Path=%s, Type=%s, Fit=%s, Name=%s\n", size, unit, path, partitionType, fit, name)
						//fmt.Println("Particiones Primarias creadas: ", primaryCount)

					} else if partitionType == "e" {
						if extendedCount >= 1 {
							response.Message = append(response.Message, "Error: Ya existe una partición extendida en el disco.")
							//fmt.Println("Error: Ya existe una partición extendida en el disco.")
							extendedCount = 0
							continue
						}

						var size1 int64
						if unit == "k" { // convertir a bytes
							size1 = int64(size) * 1024
						} else if unit == "m" { // convertir a bytes
							size1 = int64(size) * 1024 * 1024
						} else if unit == "b" {
							size1 = int64(size) * 1
						}

						partition := Partition1{
							PartStatus: '0',              // Marcar la partición como activa
							PartType:   partitionType[0], // Tipo de partición ('p' para primaria, 'e' para extendida)
							PartFit:    fit[0],           // Ajuste de la partición
							PartS:      int64(size),
						}
						copy(partition.PartName[:], name)

						// Intentar crear la partición extendida
						err := crearParticion(path, int64(size1), name, partitionType)
						if err != nil {
							response.Message = append(response.Message, fmt.Sprintf("Error al crear la partición: %s", err.Error()))
							//fmt.Println("Error al crear partición:", err)
							continue
						}

						// Incrementar extendedCount solo si no hubo errores
						extendedCount++
						response.Message = append(response.Message, fmt.Sprintf("Partición creada: Size=%d, Unit=%s, Path=%s, Type=%s, Fit=%s, Name=%s", size, unit, path, partitionType, fit, name))

						//fmt.Printf("Partición creada: Size=%d, Unit=%s, Path=%s, Type=%s, Fit=%s, Name=%s\n", size, unit, path, partitionType, fit, name)
						//fmt.Print("Particiones Extendidas creadas: ", extendedCount)

					} else if partitionType == "l" {
						var size1 int64
						if unit == "k" { // convertir a bytes
							size1 = int64(size) * 1024
						} else if unit == "m" { // convertir a bytes
							size1 = int64(size) * 1024 * 1024
						} else if unit == "b" {
							size1 = int64(size) * 1
						}

						err := crearParticionLogica(path, int64(size1), name, fit)
						if err != nil {
							response.Message = append(response.Message, fmt.Sprintf("Error al crear la partición lógica: %s", err.Error()))
							fmt.Println("Error al crear partición lógica:", err)
							continue
						}

						fmt.Printf("Partición lógica creada: Size=%d, Unit=%s, Path=%s, Type=%s, Fit=%s, Name=%s\n", size, unit, path, partitionType, fit, name)
						response.Message = append(response.Message, fmt.Sprintf("Partición lógica creada: Size=%d, Unit=%s, Path=%s, Type=%s, Fit=%s, Name=%s", size, unit, path, partitionType, fit, name))
						fmt.Println("Partición lógica creada:", name)

					} else {
						response.Message = append(response.Message, fmt.Sprintf("Error: Tipo de partición no válido: %s", partitionType))
						fmt.Println("Error: Tipo de partición no válido:", partitionType)
					}

					file, err := os.OpenFile(path, os.O_RDWR, 0666)
					if err != nil {
						response.Message = append(response.Message, fmt.Sprintf("Error al abrir el archivo: %s", err.Error()))
						return
					}
					defer file.Close()

					var mbr MBR
					if err := binary.Read(file, binary.LittleEndian, &mbr); err != nil {
						response.Message = append(response.Message, fmt.Sprintf("Error al leer el MBR: %s", err.Error()))
						return
					}
					//imprimirPartitions(file, &mbr)
				}
			} else if strings.HasPrefix(cmd, "mount") {
				isMounted, _ := isPartitionMounted(path, name)

				// Abrir el archivo del disco
				file, err := os.OpenFile(path, os.O_RDWR, 0644)
				if err != nil {
					response.Message = append(response.Message, fmt.Sprintf("Error al abrir el archivo: %s", err.Error()))
					return
				}
				defer file.Close()
				// Buscar la partición por nombre dentro del MBR
				var mbr MBR
				if err := binary.Read(file, binary.LittleEndian, &mbr); err != nil {
					response.Message = append(response.Message, fmt.Sprintf("Error al leer el MBR: %s", err.Error()))
					return
				}

				found := false
				for i := 0; i < len(mbr.Partitions); i++ {
					part := &mbr.Partitions[i]
					if strings.Trim(string(part.PartName[:]), "\x00") == name && part.PartType == 'p' {
						found = true
						break
					}
				}

				if isMounted {
					//response.Message = append(response.Message, "La partición ya está montada.")
					response.Error = fmt.Sprintf("La partición ya está montada.")
					response.Message = append(response.Message, fmt.Sprintf("La partición ya está montada."))
				} else if !found {
					response.Message = append(response.Message, fmt.Sprintf("Error: No se encontró la partición con nombre %s", name))
					//return
				} else {
					err = mountPartition(path, name, "201900603")
					if err != nil {
						response.Message = append(response.Message, fmt.Sprintf("Error al montar la particion: %s", err.Error()))
						fmt.Println("Error al montar la partición:", err)
						return
					}
					response.Message = append(response.Message, fmt.Sprintf("Partición montada: Path=%s, Name=%s", path, name))
					response.Message = append(response.Message, "Particiones montadas:")
					for id, partition := range mountedPartitions {
						response.Message = append(response.Message, fmt.Sprintf("ID: %s, Path: %s, Partición: %s, Tamaño: %d bytes, Inicio: %d", id, partition.Path, strings.Trim(string(partition.Partition.PartName[:]), "\x00"), partition.Partition.PartS, partition.Partition.PartStart))
					}
				}

			} else if strings.HasPrefix(cmd, "mkfs") {
				id, fsType, full, err := parseMkfsCommand(cmd)
				if err != nil {
					response.Message = append(response.Message, fmt.Sprintf("Error: %s", err.Error()))
					fmt.Println("Error:", err)
					continue
				}
				err = formatPartition(id, fsType, full)
				response.Message = append(response.Message, fmt.Sprintf("Partición formateada: FS=%s, Full=%t", fsType, full))
				if err != nil {
					response.Message = append(response.Message, fmt.Sprintf("Error al formatear la partición: %s", err.Error()))
					fmt.Println("Error al formatear la partición:", err)
				}
			} else if strings.HasPrefix(cmd, "login") {
				fmt.Println("Login", user, pass, id)
				fmt.Println(isLoggedIn)
				// Verificar si la partición está montada
				verificar := isPartitionMountedByID(id)
				if !verificar {
					response.Message = append(response.Message, "No se ha montado la partición")
					return
				}

				// Verificar si ya hay una sesión iniciada
				if !isLoggedIn {
					// Intentar iniciar sesión
					err := login(user, fmt.Sprintf("%v", pass), id)
					if err != nil {
						response.Message = append(response.Message, fmt.Sprintf("Error al iniciar sesión: %s", err.Error()))
						fmt.Println("Error:", err)
					} else {
						isLoggedIn = true
						response.Message = append(response.Message, "Sesión iniciada con éxito.")
					}
				} else {
					response.Message = append(response.Message, "Ya hay una sesión iniciada.")
				}

			} else if strings.HasPrefix(cmd, "logout") {
				err := logout()
				if err != nil {
					response.Message = append(response.Message, fmt.Sprintf("Error al cerrar sesión: %s", err.Error()))
					fmt.Println("Error:", err)
				} else {
					isLoggedIn = false
					response.Message = append(response.Message, "Sesión cerrada con éxito.")
				}

			} else if strings.HasPrefix(cmd, "rep") {
				//fmt.Println("ID:", id)
				if name == "mbr" {
					_, err := readMBR_ID(id)
					if err != nil {
						// Registrar el error en el mensaje de respuesta pero no finalizar la ejecución
						response.Error = fmt.Sprintf("Error al leer el MBR: %s", err)
						response.Message = append(response.Message, response.Error)
						fmt.Println("Error al leer el MBR:", err)
						continue
					}
					//Leer el EBR usando el ID y la posición de inicio de la partición
					partition := mountedPartitions[id].Partition
					fmt.Println("ID:", id)
					//fmt.Println("Particion:", partition)
					fmt.Println("Particion:", partition.PartStart)
					fmt.Println("Particion:", mountedPartitions[id].Path)

					// ebrs, err := readAllEBRs(mountedPartitions[id].Path, partition.PartStart)
					// if err != nil {
					// 	response.Error = fmt.Sprintf("Error al leer los EBRs: %s", err)
					// 	fmt.Println("Error al leer los EBRs:", err)
					// 	return
					// }
					//fmt.Println("EBRs aqui llego :", ebrs)
					// Buscar la partición y EBR dentro del MBR si es necesario
					// partitionFound, _, err := findPartitionAndEBR(mbr, id)
					// if err != nil {
					// 	response.Error = fmt.Sprintf("Error al encontrar la partición y el EBR: %s", err)
					// 	fmt.Println("Error al encontrar la partición y el EBR:", err)
					// 	return
					// }
					// fmt.Println("Particion encontrada:", partitionFound)
					// fmt.Println("Particion encontrada:", path)

					//Generar el reporte del MBR y EBR
					err = Report_MBR_EBRs(mountedPartitions[id].Path, path)
					fmt.Println("Reporte Generado:", path)
					if err != nil {
						response.Error = fmt.Sprintf("Error al generar el reporte: %s", err)
						return
					}
					//fmt.Println(mbr, ebrs, partitionFound, mountedPartitions[id].Path, path)
					response.Message = append(response.Message, fmt.Sprintf("Reporte generado: %s", name))
					if err != nil {
						response.Error = fmt.Sprintf("Error al generar el reporte: %s", err)
						fmt.Println("Error al generar el reporte:", err)
					}
				} else if name == "disk" {
					mbr, err := readMBR_ID(id)
					if err != nil {
						// Registrar el error en el mensaje de respuesta pero no finalizar la ejecución
						response.Error = fmt.Sprintf("Error al leer el MBR: %s", err)
						response.Message = append(response.Message, response.Error)
						fmt.Println("Error al leer el MBR:", err)

						// Continuar con la ejecución, no usar return o finalizar aquí
						// El uso de `continue` aquí es para seguir con otros comandos o pasos si estás en un bucle
						continue
					}
					fmt.Print(mbr)
					err = generateDiskReport(mbr, path, name)
					if err != nil {
						response.Error = fmt.Sprintf("Error al generar el reporte: %s", err)
						return
					}
					response.Message = append(response.Message, fmt.Sprintf("Reporte generado: %s", name))
					if err != nil {
						response.Error = fmt.Sprintf("Error al generar el reporte: %s", err)
						fmt.Println("Error al generar el reporte:", err)
					}
				} else if name == "sb" {
					//fmt.Println("ID:", id)
					superblock, err := LeerSuperBloquePorID(id)
					if err != nil {
						response.Error = fmt.Sprintf("Error al leer el Super Bloque: %s", err)
						return
					}
					//fmt.Println("Reporte Generado:", path)
					Report_SuperBlock(*superblock, path)

					response.Message = append(response.Message, fmt.Sprintf("Reporte Generado: %s", name))
					if err != nil {
						response.Error = fmt.Sprintf("Reporte no generado: %s", name)
						fmt.Println("Reporte no generado:", name)
					}
				} else if name == "inode" {
					response.Message = append(response.Message, fmt.Sprintf("Reporte Generado: %s", name))
					if err != nil {
						response.Error = fmt.Sprintf("Reporte no generado: %s", name)
						fmt.Println("Reporte no generado:", name)
					}
				} else if name == "block" {
					response.Message = append(response.Message, fmt.Sprintf("Reporte Generado: %s", name))
					if err != nil {
						response.Error = fmt.Sprintf("Reporte no generado: %s", name)
						fmt.Println("Reporte no generado:", name)
					}
				} else if name == "bm_inode" {
					response.Message = append(response.Message, fmt.Sprintf("Reporte Generado: %s", name))
					if err != nil {
						response.Error = fmt.Sprintf("Reporte no generado: %s", name)
						fmt.Println("Reporte no generado:", name)
					}
				} else if name == "bm_bloc" {
					response.Message = append(response.Message, fmt.Sprintf("Reporte Generado: %s", name))
					if err != nil {
						response.Error = fmt.Sprintf("Reporte no generado: %s", name)
						fmt.Println("Reporte no generado:", name)
					}
				} else if name == "file" {
					response.Message = append(response.Message, fmt.Sprintf("Reporte Generado: %s", name))
					if err != nil {
						response.Error = fmt.Sprintf("Reporte no generado: %s", name)
						fmt.Println("Reporte no generado:", name)
					}
				} else if name == "ls" {
					response.Message = append(response.Message, fmt.Sprintf("Reporte Generado: %s", name))
					if err != nil {
						response.Error = fmt.Sprintf("Reporte no generado: %s", name)
						fmt.Println("Reporte no generado:", name)
					}
				} else {
					response.Error = fmt.Sprintf("Reporte no generado: %s", name)
					fmt.Println("Reporte no generado:", name)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	} else {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
	}
}

func printMountedPartitions() {
	response := Response{}
	if len(mountedPartitions) == 0 {
		fmt.Println("No hay particiones montadas.")
		response.Message = append(response.Message, "No hay particiones montadas.")
		return
	}

	fmt.Println("Particiones montadas:")
	response.Message = append(response.Message, "Particiones montadas:")
	for id, partition := range mountedPartitions {
		fmt.Printf("ID: %s, Path: %s, Partición: %s, Tamaño: %d bytes, Inicio: %d\n",
			id, partition.Path, strings.Trim(string(partition.Partition.PartName[:]), "\x00"),
			partition.Partition.PartS, partition.Partition.PartStart)

		response.Message = append(response.Message, fmt.Sprintf("ID: %s, Path: %s, Partición: %s, Tamaño: %d bytes, Inicio: %d\n", id, partition.Path, strings.Trim(string(partition.Partition.PartName[:]), "\x00"), partition.Partition.PartS, partition.Partition.PartStart))
	}
}

// Función para leer el MBR desde el archivo usando solo el ID montado
func readMBR_ID(id string) (MBR, error) {
	response := Response{}
	// Buscar la partición montada por ID
	partition, exists := mountedPartitions[id]
	if !exists {
		err := fmt.Errorf("partición con ID '%s' no está montada", id)
		response.Message = append(response.Message, err.Error())
		fmt.Println("Error:", err) // Imprime el error para depuración
		return MBR{}, err
	}

	//fmt.Print(partition.Path)
	//fmt.Println("readMBR_ID")

	// Usar el path de la partición montada
	file, err := os.Open(partition.Path)
	if err != nil {
		err = fmt.Errorf("error al abrir el archivo del disco: %v", err)
		response.Message = append(response.Message, err.Error())
		fmt.Println("Error:", err) // Imprime el error para depuración
		return MBR{}, err
	}
	defer file.Close()

	var mbr MBR
	mbrData := make([]byte, binary.Size(mbr))
	_, err = file.ReadAt(mbrData, 0)
	if err != nil {
		err = fmt.Errorf("error al leer el MBR del disco: %v", err)
		response.Message = append(response.Message, err.Error())
		fmt.Println("Error:", err) // Imprime el error para depuración
		return MBR{}, err
	}

	buffer := bytes.NewBuffer(mbrData)
	if err := binary.Read(buffer, binary.LittleEndian, &mbr); err != nil {
		err = fmt.Errorf("error al decodificar el MBR: %v", err)
		response.Message = append(response.Message, err.Error())
		fmt.Println("Error:", err) // Imprime el error para depuración
		return MBR{}, err
	}

	return mbr, nil
}
func readEBR_ID(id string, start int64) (*EBR, error) {
	// Buscar la partición montada por ID
	partition, exists := mountedPartitions[id]
	if !exists {
		return nil, fmt.Errorf("partición con ID '%s' no está montada", id)
	}

	// Usar el path de la partición montada
	file, err := os.Open(partition.Path)
	if err != nil {
		return nil, fmt.Errorf("error al abrir el archivo del disco: %v", err)
	}
	defer file.Close()

	var ebr EBR
	if _, err := file.Seek(start, 0); err != nil {
		return nil, fmt.Errorf("error al posicionarse en el archivo: %v", err)
	}

	if err := binary.Read(file, binary.LittleEndian, &ebr); err != nil {
		return nil, fmt.Errorf("error al leer el EBR: %v", err)
	}

	return &ebr, nil
}

//*********************************************************************

func getDiscosHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Solicitud GET recibida en /discos")
	if r.Method == http.MethodGet {
		mutex.Lock()
		defer mutex.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(disks)
	}
}

func withCORS(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // Permitir cualquier origen
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		handler(w, r)
	}
}

func main() {
	http.HandleFunc("/execute", withCORS(executeHandler))  // POST para crear discos
	http.HandleFunc("/discos", withCORS(getDiscosHandler)) // GET para obtener discos
	// http.HandleFunc("/discos/eliminar", deleteDiskHandler) // POST para eliminar discos

	fmt.Println("Server running on port 8080...")
	http.ListenAndServe(":8080", nil)
}
