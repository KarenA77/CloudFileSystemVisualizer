package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type SuperBlock struct {
	FilesystemType  int32    // s_filesystem_type: Identificador del sistema de archivos
	InodesCount     int32    // s_inodes_count: Número total de inodos
	BlocksCount     int32    // s_blocks_count: Número total de bloques
	FreeBlocksCount int32    // s_free_blocks_count: Número de bloques libres
	FreeInodesCount int32    // s_free_inodes_count: Número de inodos libres
	MountTime       [19]byte // s_mtime: Última fecha en la que el sistema fue montado (timestamp)
	UnmountTime     [19]byte // s_umtime: Última fecha en que el sistema fue desmontado (timestamp)
	MountCount      int32    // s_mnt_count: Número de veces que se ha montado el sistema
	Magic           int32    // s_magic: Identificador del sistema de archivos, debe ser 0xEF53
	InodeSize       int32    // s_inode_size: Tamaño del inodo
	BlockSize       int32    // s_block_size: Tamaño del bloque
	FirstInode      int32    // s_first_inode: Primer inodo libre
	FirstBlock      int32    // s_first_block: Primer bloque libre
	BmInodeStart    int32    // s_bm_inode_start: Inicio del bitmap de inodos
	BmBlockStart    int32    // s_bm_block_start: Inicio del bitmap de bloques
	InodeStart      int32    // s_inode_start: Inicio de la tabla de inodos
	BlockStart      int32    // s_block_start: Inicio de la tabla de bloques
}

// type SuperBlock struct {
// 	FilesystemType int32 // Tipo de sistema de archivos (ext2 = 2, ext3 = 3)
// 	MagicNumber    int32 // Número mágico para validar el sistema de archivos
// 	PartSize       int64 // Tamaño de la partición
// }

type actual_user struct {
	Uid int32
	Gid int32
	Grp string
	Usr string
	Pwd string
	Pid string
}

type Inode struct {
	UID    int       // i_uid: UID del usuario propietario del archivo o carpeta
	GID    int       // i_gid: GID del grupo al que pertenece el archivo o carpeta
	Size   int       // i_size: Tamaño del archivo en bytes
	ATIME  time.Time // i_atime: Última fecha en que se leyó el inodo sin modificarlo
	CTIME  time.Time // i_ctime: Fecha en la que se creó el inodo
	MTIME  time.Time // i_mtime: Última fecha en la que se modifica el inodo
	Blocks [16]int   // i_block: Array de bloques, 12 directos, 1 simple indirecto, 1 doble indirecto, 1 triple indirecto
	Type   byte      // i_type: Tipo de archivo (1 = Archivo, 0 = Carpeta)
	Perms  [3]byte   // i_perm: Permisos del archivo o carpeta (octal)
}

const (
	FileType = 1
	DirType  = 0
)

// Permisos para `Perms` en octal
const (
	Read  = 0b100
	Write = 0b010
	Exec  = 0b001
)

type User struct {
	ID       int
	Username string
	Password string
	Group    string
}

// BlockContent representa el contenido dentro de un bloque de carpeta.
type BlockContent struct {
	Name  [12]byte
	Inode int
}

// DirectoryBlock representa la estructura de un bloque de carpeta en un sistema de archivos EXT2.
type DirectoryBlock struct {
	Content [4]BlockContent
}

// BLOQUES DE ARCHIVOS
type FolderBlock struct {
	B_content [4]BlockContent
}

type Fileblock struct {
	B_content [64]byte
}

/*-----------------------------------Administración del Sistema de Archivos-----------------------------------*/
/*----------------------------------------------MKFS----------------------------------------------*/
//Analiza el comando MKFS y extrae los parámetros
func parseMkfsCommand(command string) (id, fsType string, full bool, err error) {
	parts := strings.Fields(command)
	cleanedCommand := strings.TrimSpace(command)
	cleanedCommand = strings.ToLower(cleanedCommand)

	fsType = "ext2" // Por defecto se utiliza ext2
	full = false

	for _, part := range parts {
		if strings.HasPrefix(part, "-id=") {
			id = strings.TrimPrefix(part, "-id=")
		} else if strings.HasPrefix(part, "-type=") {
			fsType = strings.TrimPrefix(part, "-type=")
			if fsType == "full" {
				full = true
				fsType = "ext2" // Mantener el formato predeterminado en caso de que no se especifique
			}
		}
	}

	if id == "" {
		err = fmt.Errorf("id de la partición no especificado")
	}
	return
}

// Crea el archivo users.txt dentro de la partición formateada
func createUsersFile(file *os.File, blockStart int) error {
	// Contenido inicial de users.txt
	usersContent := "1,G,root\n1,U,root,root,123\n"

	// Posicionarse en el inicio del bloque donde se escribirá users.txt
	if _, err := file.Seek(int64(blockStart), 0); err != nil {
		return fmt.Errorf("Error al posicionarse para escribir users.txt: %v", err)
	}
	// Escribir el contenido de users.txt
	if _, err := file.Write([]byte(usersContent)); err != nil {
		return fmt.Errorf("Error al escribir el archivo users.txt: %v", err)
	}

	return nil
}

func initializeBitmaps(file *os.File, superblock *SuperBlock) error {
	// Inicializar mapas de bits a 0 (libres)
	bitmapInodes := make([]byte, superblock.InodesCount)
	bitmapBlocks := make([]byte, superblock.BlocksCount)

	// Escribir el mapa de bits de inodos
	if _, err := file.Seek(int64(superblock.BmInodeStart), 0); err != nil {
		return fmt.Errorf("Error al posicionarse en el bitmap de inodos: %v", err)
	}
	if _, err := file.Write(bitmapInodes); err != nil {
		return fmt.Errorf("Error al escribir el bitmap de inodos: %v", err)
	}

	// Escribir el mapa de bits de bloques
	if _, err := file.Seek(int64(superblock.BmBlockStart), 0); err != nil {
		return fmt.Errorf("Error al posicionarse en el bitmap de bloques: %v", err)
	}
	if _, err := file.Write(bitmapBlocks); err != nil {
		return fmt.Errorf("Error al escribir el bitmap de bloques: %v", err)
	}

	return nil
}

// Formatea la partición y crea el archivo users.txt
func formatPartition(id, fsType string, full bool) error {
	id = strings.ToLower(id)

	// Buscar la partición montada por ID
	partition, exists := mountedPartitions[id]
	if !exists {
		return fmt.Errorf("Error al formatear la partición: partición con ID '%s' no está montada", id)
	}

	// Validar si la partición es primaria
	if partition.Partition.PartType != 'p' {
		return fmt.Errorf("Error al formatear la partición: la partición con ID '%s' no es primaria", id)
	}

	// Abrir el archivo del disco
	file, err := os.OpenFile(partition.Path, os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("Error al abrir el disco: %v", err)
	}
	defer file.Close()

	// Calcular los tamaños de las estructuras
	sizeOfSuperblock := binary.Size(SuperBlock{})
	sizeOfInodo := binary.Size(Inode{})
	sizeOfBloque := binary.Size(Fileblock{})

	// Tamaño de la partición
	partitionSize := int(partition.Partition.PartS)
	numerator := partitionSize - sizeOfSuperblock
	denominator := 4 + sizeOfInodo + 3*sizeOfBloque
	n := numerator / denominator
	if n < 1 {
		n = 1
	}

	inodeCount := n
	blockCount := 3 * n

	var superblock SuperBlock
	if fsType == "ext2" {
		superblock = SuperBlock{
			FilesystemType:  2,
			InodesCount:     int32(inodeCount),
			BlocksCount:     int32(blockCount),
			FreeBlocksCount: int32(blockCount),
			FreeInodesCount: int32(inodeCount),
			MountCount:      1,
			Magic:           0xEF53,
			InodeSize:       int32(sizeOfInodo),
			BlockSize:       int32(sizeOfBloque),
			BmInodeStart:    int32(int(partition.Partition.PartStart) + sizeOfSuperblock),
			BmBlockStart:    int32(int(partition.Partition.PartStart) + sizeOfSuperblock + inodeCount),
			InodeStart:      int32(int(partition.Partition.PartStart) + sizeOfSuperblock + inodeCount + blockCount),
			BlockStart:      int32(int(partition.Partition.PartStart) + sizeOfSuperblock + inodeCount + blockCount + (inodeCount * sizeOfInodo)),
		}
		copy(superblock.MountTime[:], time.Now().Format("2006-01-02 15:04:05"))
		copy(superblock.UnmountTime[:], time.Now().Format("2006-01-02 15:04:05"))
	} else {
		return fmt.Errorf("Tipo de sistema de archivos no soportado: %s", fsType)
	}

	// Escribir el superblock en el inicio de la partición
	if _, err := file.Seek(partition.Partition.PartStart, 0); err != nil {
		return fmt.Errorf("Error al posicionarse en el inicio de la partición: %v", err)
	}
	if err := binary.Write(file, binary.LittleEndian, &superblock); err != nil {
		return fmt.Errorf("Error al escribir el superblock: %v", err)
	}

	// Inicializar mapas de bits de inodos y bloques
	if err := initializeBitmaps(file, &superblock); err != nil {
		return fmt.Errorf("Error al inicializar los mapas de bits: %v", err)
	}

	// Crear el archivo users.txt con el contenido inicial
	if err := createUsersFile(file, int(superblock.BlockStart)); err != nil {
		return fmt.Errorf("Error al crear el archivo users.txt: %v", err)
	}

	fmt.Printf("Partición con ID '%s' formateada a %s con éxito.\n", id, fsType)
	return nil
}

/*----------------------------------------------LOGIN----------------------------------------------*/
var users = make(map[string]User)
var sesionActiva = false
var usuarioActual string
var idSesionActual int

// Analiza el comando LOGIN y extrae los parámetros
func parseLoginCommand(command string) (username, password, id string, err error) {
	parts := strings.Fields(command)
	cleanedCommand := strings.SplitN(command, "#", 2)[0] // Ignorar comentarios si existen
	cleanedCommand = strings.TrimSpace(cleanedCommand)

	// Iterar sobre cada parte del comando para encontrar los parámetros
	for _, part := range parts {
		if strings.HasPrefix(part, "-user=") {
			username = strings.TrimPrefix(part, "-user=")
			// Manejo de comillas alrededor del nombre de usuario
			if len(username) > 0 && username[0] == '"' && username[len(username)-1] == '"' {
				username = username[1 : len(username)-1] // Remover comillas al inicio y al final
			} else {
				// Intentar capturar el nombre de usuario con expresión regular si no se encuentra de forma directa
				re := regexp.MustCompile(`-user="([^"]+)"`)
				matches := re.FindStringSubmatch(cleanedCommand)
				if len(matches) > 1 {
					username = matches[1]
				}
			}
		} else if strings.HasPrefix(part, "-pass=") {
			password = strings.TrimPrefix(part, "-pass=")
			// Manejo de comillas alrededor de la contraseña
			if len(password) > 0 && password[0] == '"' && password[len(password)-1] == '"' {
				password = password[1 : len(password)-1]
			} else {
				re := regexp.MustCompile(`-pass="([^"]+)"`)
				matches := re.FindStringSubmatch(cleanedCommand)
				if len(matches) > 1 {
					password = matches[1]
				}
			}
		} else if strings.HasPrefix(part, "-id=") {
			id = strings.TrimPrefix(part, "-id=")
		}
	}

	// Validar que se haya especificado el ID
	if id == "" {
		err = fmt.Errorf("id de la partición no especificado")
	}
	return
}
func isPartitionMountedByID(id string) bool {
	// Convertir el ID a minúsculas para asegurar consistencia en la búsqueda
	id = strings.ToLower(id)

	// Verificar si el ID existe en el mapa de particiones montadas
	_, exists := mountedPartitions[id]

	return exists
}

// Función para cargar usuarios desde users.txt
func loadUsers(file *os.File, blockStart int) error {
	if _, err := file.Seek(int64(blockStart), 0); err != nil {
		return fmt.Errorf("Error al posicionarse para leer users.txt: %v", err)
	}

	content := make([]byte, 1024)
	n, err := file.Read(content)
	if err != nil {
		return fmt.Errorf("Error al leer users.txt: %v", err)
	}
	fmt.Printf("Contenido leído de users.txt: %s\n", string(content[:n]))
	lines := strings.Split(strings.TrimSpace(string(content[:n])), "\n")
	for _, line := range lines {
		parts := strings.Split(line, ",")
		if len(parts) == 3 && parts[1] == "G" {
			continue
		} else if len(parts) == 5 && parts[1] == "U" {
			id, _ := strconv.Atoi(parts[0])
			username := strings.TrimSpace(parts[3])
			users[username] = User{
				ID:       id,
				Username: username,
				Password: strings.TrimSpace(parts[4]),
				Group:    strings.TrimSpace(parts[2]),
			}
			// Depuración: Verificar los usuarios cargados
			fmt.Printf("Usuario cargado: %+v\n", users[username])
		}
	}
	return nil
}

// Función de inicio de sesión
func login(username, password string, id string) error {
	// Verificar si ya hay una sesión activa
	if sesionActiva {
		return errors.New("ya hay una sesión activa, cierre sesión antes de iniciar una nueva")
	}

	// Verificar si el usuario existe en el mapa cargado desde users.txt
	user, exists := users[username]
	if !exists {
		fmt.Println("Usuarios cargados:", users) // Depuración: Verifica el contenido del mapa de usuarios
		return errors.New("usuario no encontrado")
	}

	// Verificar la contraseña
	if user.Password != password {
		return errors.New("contraseña incorrecta")
	}

	// Marcar la sesión como activa
	sesionActiva = true
	usuarioActual = username
	idSesionActual = user.ID

	fmt.Printf("Sesión iniciada con éxito. Bienvenido, %s.\n", username)
	return nil
}

/*----------------------------------------------LOGOUT----------------------------------------------*/
// Analiza el comando rmdisk y extrae el parámetro de la ruta.
func parseLogoutCommand(command2 string) (log string, err error) {
	parts := strings.Fields(strings.ToLower(command2))
	cleanedCommand := strings.SplitN(command2, "#", 2)[0] // Ignorar comentarios si existen
	cleanedCommand = strings.TrimSpace(cleanedCommand)
	cleanedCommand = strings.ToLower(cleanedCommand)
	//fmt.Println("ComandoRM:", cleanedCommand)
	//fmt.Println("PartsRM:", parts)

	// Buscar la parte que empieza con "-path="
	for _, part := range parts {
		if strings.HasPrefix(part, "logout") {
			log = strings.TrimPrefix(part, "logout")
		}

	}

	if log == "" {
		err = fmt.Errorf("Log no especificado")
	}
	return log, nil
}

// Cierra la sesión actual
func logout() error {
	if !sesionActiva {
		return errors.New("no hay una sesión activa para cerrar")
	}

	// Resetea la sesión
	sesionActiva = false
	usuarioActual = ""
	idSesionActual = 0

	fmt.Println("Sesión cerrada con éxito.")
	return nil
}
