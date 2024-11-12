# CloudFileSystemVisualizer
Este proyecto implementa un visualizador de sistemas de archivos en la nube, proporcionando una interfaz web que permite explorar discos, particiones y archivos en un sistema de archivos compatible con EXT3. Desarrollado como un sistema de gestión accesible en Amazon Web Services (AWS), el proyecto incluye un frontend basado en S3 y un backend en Go alojado en una instancia EC2. La aplicación permite explorar el sistema de archivos en modo solo lectura y ofrece funcionalidades avanzadas como la creación, edición, eliminación y navegación a través de comandos.

Requerimientos Mínimos
Tecnologías utilizadas
* Frontend
    * React: desplegado en S3.
    * Backend: API Rest en Go, desplegado en EC2 (Linux, en Ubuntu).
* Servicios de AWS
    * S3 para almacenamiento estático
    * EC2 para el backend.
  
