# 📘 Manual Técnico: GoDisk 2.0  
### **Proyecto MIA 2S2025**

#### Universidad San Carlos de Guatemala  
#### Facultad de Ingeniería – Escuela de Ciencias y Sistemas  
#### Curso: Manejo e Implementación de Archivos  

**Proyecto:** GoDisk – Simulador de Sistema de Archivos EXT2/EXT3  
**Estudiante:** Jairo Adelso Gomez Hernandez — *201902672*  
**Fecha:** Guatemala, Octubre 2025

---

## 1. Introducción
Este documento describe el **diseño técnico**, las **estructuras internas** y las **decisiones de implementación** de **GoDisk 2.0**, un simulador de sistemas de archivos **EXT2 y EXT3** con backend en **Go** y **frontend web**.  
En la **Fase 2** el sistema se desplegó en **AWS**: **EC2** para el backend (API en Go Fiber) y **S3** como **sitio web estático** para el frontend.

---

## 2. Arquitectura del Sistema

### 2.1 Vista de Alto Nivel
```
Usuario ── navegador ──> Frontend (S3 Static Website)
                            │
                            ▼  HTTPS (CORS)
                      Backend API (EC2, Go Fiber)
                            │
                            ▼
                   Archivos .mia (discos virtuales)
                       + Reportes Graphviz
```

- **Frontend (S3):** Interfaz con terminal embebida, carga de scripts `.smia`, visor de reportes.  
- **Backend (EC2, Go Fiber):** Expone `/health` y `/execute`. Interpreta y ejecuta comandos contra archivos `.mia`.  
- **Persistencia:** Todo se escribe y lee **directo del archivo binario** del disco virtual.


### 2.2 Flujo de Ejecución End-to-End
1. El usuario ingresa comandos o sube un script `.smia` desde el frontend.  
2. El frontend envía un `POST /execute` con el texto.  
3. `main.go` divide por líneas, ignora comentarios `#` y delega a `Analizador.AnalyzerCommand`.  
4. El **Analizador** identifica el comando, **parsea parámetros** y llama al **módulo** correspondiente.  
5. El módulo lee/escribe estructuras en el archivo `.mia`.  
6. Se devuelve salida formateada (éxito/error), y si aplica, se generan reportes con **Graphviz**.

### 2.3 API del Backend (Go Fiber)
- `GET /health` → *healthcheck simple* (“OK”).  
- `POST /execute` → **Entrada** `{ "command": "líneas de comandos" }` → **Salida** `{ "output": "texto de respuesta" }`.

**Fragmento (`main.go`):**
```go
app.Post("/execute", func(c *fiber.Ctx) error {
    var req CommandRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(CommandResponse{Output: "Error: Petición inválida"})
    }
    script := strings.ReplaceAll(req.Command, "
", "
")
    lines := strings.Split(script, "
")
    var out bytes.Buffer
    for _, raw := range lines {
        line := strings.TrimSpace(raw)
        if line == "" { out.WriteString("\n"); continue }
        if strings.HasPrefix(line, "#") { out.WriteString(line + "\n"); continue }
        cmd, params := Analizador.GetInput(line)
        if cmd == "" { out.WriteString("Error: línea no reconocida\n"); continue }
        result := Analizador.AnalyzerCommand(cmd, params)
        if !strings.HasSuffix(result, "\n") { result += "\n" }
        out.WriteString(result)
    }
    output := strings.TrimRight(out.String(), "\n") + "\n"
    if strings.TrimSpace(output) == "" { output = "No se ejecutó ningún comando\n" }
    return c.JSON(CommandResponse{Output: output})
})
```

> **CORS:** habilitado para `*` (útil en despliegue S3 ↔ EC2).

---

## 3. Análisis Léxico y Sintáctico de Comandos

### 3.1 Tokenización básica
```go
// Analizador/Analizador.go
var paramRegex = regexp.MustCompile(`-(\w+)=("[^"]+"|\S+)`)
func GetInput(input string) (string, string) { /* separa comando y params */ }
func ExtractParams(params string) map[string]string {
    matches := paramRegex.FindAllStringSubmatch(params, -1)
    // construye map[string]string con flags normalizadas
}
```

### 3.2 Enrutamiento por comando
```go
func AnalyzerCommand(commands string, params string) string {
    result := "> " + commands + " " + params + "\n"
    switch {
    case strings.Contains(commands, "mkdisk"):
        return result + fn_mkdisk(params)
    case strings.Contains(commands, "fdisk"):
        return result + fn_fdisk(params)
    case strings.Contains(commands, "mount"):
        return result + fn_mount(params)
    case strings.Contains(commands, "mkfs"):
        return result + fn_mkfs(params)
    case strings.Contains((commands), "cat"):
        return result + fn_cat(params)
    default:
        return result + "Error: Comando inválido o no encontrado"
    }
}
```

---

## 4. Persistencia Binaria y Utilidades

### 4.1 I/O Binario
```go
// Utils/Utils.go
func WriteFile(file *os.File, data interface{}, position int64) error {
    if _, err := file.Seek(position, 0); err != nil { return fmt.Errorf("seek: %v", err) }
    if err := binary.Write(file, binary.LittleEndian, data); err != nil { return fmt.Errorf("write: %v", err) }
    return nil
}
func ReadFile(file *os.File, data interface{}, position int64) error {
    if _, err := file.Seek(position, 0); err != nil { return fmt.Errorf("seek: %v", err) }
    if err := binary.Read(file, binary.LittleEndian, data); err != nil { return fmt.Errorf("read: %v", err) }
    return nil
}
```

### 4.2 Carga a S3 (para recursos/reportes opcional)
```go
// Utils/Utils.go
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

---

## 5. Estructuras de Datos (EXT2/EXT3)

> Declaradas en `Particiones/*.go`. A continuación, un resumen de las más relevantes:

### 5.1 MBR y Particiones
```go
type Partition struct {
    Part_Status [1]byte
    Part_Type   [1]byte // 'p','e','l'
    Part_Fit    [1]byte
    Part_Start  int32
    Part_Size   int32
    Part_Name   [16]byte
    Part_Correlative int32
    Part_ID     [4]byte
}
```

### 5.2 Superbloque
```go
type SuperBlock struct {
    S_filesystem_type   int32 // 2=EXT2, 3=EXT3
    S_inodes_count      int32
    S_blocks_count      int32
    S_free_blocks_count int32
    S_free_inodes_count int32
    S_mtime, S_unmtime  [17]byte
    S_mnt_count, S_magic, S_inode_size, S_block_size int32
    S_first_ino, S_first_blo                         int32
    S_bm_inode_start, S_bm_block_start              int32
    S_inode_start, S_block_start                    int32
}
```

### 5.3 Inodos y Bloques
```go
type Inode struct {
    I_uid, I_gid, I_size int32
    I_atime, I_ctime, I_mtime [17]byte
    I_block [15]int32 // 12 directos + ind. S/D/T
    I_type  [1]byte   // '0' carpeta, '1' archivo
    I_perm  [3]byte   // 'rwx' en octal: 775, 664, etc.
}
type FolderBlock struct { B_content [4]Content }
type FileBlock   struct { B_content [64]byte }
```

### 5.4 Journaling (EXT3 – Fase 2)
- **Journal**: registro secuencial de operaciones (op, ruta, contenido, fecha/hora).  
- **Information**: payload de cada entrada.  
- **Uso**: permite **recuperación** (rollback/roll-forward) y **auditoría**.

> *Nota*: En esta versión, el journal se persiste en bloques reservados por diseño (ver enunciado).

---

## 6. Implementación de Comandos (resumen técnico)

### 6.1 `mkdisk` / `rmdisk`
- **mkdisk** crea un binario `.mia` del tamaño solicitado y escribe un **MBR** con `fit` global.  
- **rmdisk** elimina el archivo (con confirmación en UI si aplica).

### 6.2 `fdisk`
- Crea/elimina/ajusta particiones (primaria/extendida/lógica).  
- Estrategias **FF/BF/WF** para ubicación.  
- Cálculo de huecos y validaciones (máximo 4 prim/extendidas, una extendida por disco, lógicas dentro de extendida).

### 6.3 `mount` / `mounted`
- Monta una partición y asigna **ID correlativo**.  
- `mounted` lista las particiones activas (útil para `mkfs`).

### 6.4 `mkfs` (EXT2/EXT3)
- Prepara bitmaps, áreas de **inodos** y **bloques**, y **superbloque**.  
- **Crea `/` y `users.txt`**.  
- EXT3 reserva **área de journal**.

**Cálculo de `n` clásico (EXT2):**
```
n = floor( (PartitionSize - sizeof(SB)) / (4 + sizeof(Inode) + 3*sizeof(Block)) )
```
> En EXT3 se agrega la reserva del **Journal** (constante y bloques de contenido).

### 6.5 `login/logout`, `mkgrp/rmgrp`, `mkusr/rmusr`, `chgrp`
- Gestión de `users.txt` y autenticación en memoria de sesión.  
- Solo **root** puede crear/eliminar grupos/usuarios.

### 6.6 `mkdir` (permisos y recursividad `-p`)
```go
// AdmPermisos/mkdir.go (extracto)
if _, ok := params["p"]; ok { createDirectoriesInExt2(dirPath, file, sb) }
hasPerm, _ := hasPermissionInExt2(dirPath, file, sb, currentUser, "w")
// new inode + folder block (., ..), bitmap updates, enlazar en padre
```
- Mantiene **permisos 775** por defecto (root: 777).

### 6.7 `mkfile` (contenido: `-size` o `-cont`)
```go
// AdmPermisos/mkfile.go (extracto)
if contPath, ok := params["cont"]; ok {
    content = os.ReadFile(contPath)
} else if sizeStr, ok := params["size"]; ok {
    // genera contenido determinístico hasta 12*64 bytes
}
// asigna bloques libres, escribe FileBlock, actualiza inode y bitmaps
```
- Si existe, **sobrescribe limpiamente** con `freeInodeAndBlocks`.

### 6.8 `cat`
- Lee **bloques** del inode y concatena para mostrar al usuario.

### 6.9 `rep` (mbr, disk, inode, block, sb, file, ls, tree)
- Construye **DOT** y lo renderiza a `.jpg` (Graphviz).  
- `ls`: lista entradas de directorio; `tree`: recorre jerarquía de inodos/bloques.

---

## 7. Algoritmos y Utilidades Críticas

### 7.1 Asignación de recursos
```go
func findFreeInode(sb SuperBlock, f *os.File) (int32, error) {
    for i:=int32(0); i<sb.S_inodes_count; i++ {
        var bit byte
        Utils.ReadFile(f, &bit, int64(sb.S_bm_inode_start+i))
        if bit == 0 { return i, nil }
    }
    return -1, fmt.Errorf("no hay inodos libres")
}
func findFreeBlock(sb SuperBlock, f *os.File) (int32, error) { /* análogo */ }
```

### 7.2 Enlace en directorio padre
```go
func addFileToDirectory(dirPath, name string, inodeIdx int32, f *os.File, sb SuperBlock) error {
    // Busca un FolderBlock con slot libre; si no existe, asigna uno nuevo y lo enlaza
}
```

### 7.3 Limpieza segura (sobrescritura)
```go
func freeInodeAndBlocks(inodeIdx int32, f *os.File, sb SuperBlock) error {
    // Limpia FileBlocks y marca bitmap=0; luego limpia el Inode y bitmap de inodos
}
```

---

## 8. Permisos y Seguridad
- **I_perm** se interpreta como **octal** (u/g/o).  
- Verificación de permiso **R/W/X** según (propietario, grupo, otros).  
- Acceso privilegiado **root** (777).  
- Validaciones: sesión activa para operaciones que mutan estado.

---

## 9. Reportes (Graphviz)
- **Entrada**: estructuras leídas del disco (`SuperBlock`, `Inode`, `FolderBlock`, etc.).  
- **Salida**: `.dot` + `.jpg`.  
- Prácticas: sanitizar nombres, normalizar rutas, validar existencia de partición/ID.

### Consola Principal  
![Consola](./imgs/consola.png)

### Seccion de Reportes
![Reportes](./imgs/reportes.png)

### Reporte MBR  
![Reporte MBR](./imgs/mbr.jpg)

### Reporte Disco  
![Reporte Disco](./imgs/disco.png)

### Reporte de Árbol (Tree)  
![Reporte Tree](./imgs/tree.jpg)

### Reporte LS  
![Reporte LS](./imgs/ls.jpg)

### Instancia EC2
![EC2](./imgs/ec2.png)

### Bucket S3
![S3](./imgs/s3.png)
![S3](./imgs/s31.png)

### Resultado Nube
![AWS](./imgs/consola.png)

---

---

## 10. Fase 2 – Despliegue en AWS

### 10.1 Backend en EC2
- Ubuntu 22.04, Go instalado, repositorio clonado.  
- Ejecutar como servicio (opcional):
```ini
# /etc/systemd/system/godisk.service
[Unit]
Description=GoDisk API
After=network.target

[Service]
WorkingDirectory=/opt/godisk
ExecStart=/usr/bin/go run main.go
Restart=always
Environment=PORT=3001

[Install]
WantedBy=multi-user.target
```
```
sudo systemctl daemon-reload
sudo systemctl enable --now godisk
```

- **Security Group:** abrir puerto `3001/TCP` (o detrás de Nginx/ALB).  
- **CORS:** permitido en `main.go`.

### 10.2 Frontend en S3 (Sitio Web Estático)
- Compilar `build/` y subir a S3.  
- Habilitar hosting estático y política pública de solo lectura.  
- (Opcional) **CloudFront** para HTTPS y caching.

📸 *[Espacio para capturas: EC2, SGs, S3, URL del sitio]*

---

## 11. Pruebas y Validación

### 11.1 Smoke Test
1. `GET /health` → “OK”.  
2. `POST /execute` con `mkdisk`, `fdisk`, `mount`, `mkfs`, `login`.  
3. `mkdir`, `mkfile`, `cat`.  
4. `rep -name=tree` y `rep -name=ls` → validar imágenes.

### 11.2 Casos Límites
- Sin sesión al crear archivos/carpetas → **error esperado**.  
- Sobrescritura `mkfile` sobre archivo existente → libera y reescribe.  
- `mkdir` sin `-p` cuando padres no existen → **error**.  
- Bitmaps coherentes tras muchas operaciones.

---

## 12. Troubleshooting

- **“No hay Particiones Montadas”**: verificar `mount` y `mkfs`.  
- **Permisos denegados**: usuario no root y sin `w` en directorio padre.  
- **Reportes vacíos**: confirmar Graphviz instalado y rutas de salida válidas.  
- **CORS** desde S3: confirmar `AllowOrigins="*"` en backend y política pública S3.  
- **EC2 inaccesible**: revisar Security Group/VPC y firewall del SO.

---

## 13. Conclusiones Técnicas
- El diseño **modular** (analizador → comando → utilidades) favorece pruebas y extensión.  
- La **persistencia binaria** reproduce fielmente el comportamiento de EXT2/EXT3 a nivel educativo.  
- La **Fase 2 en AWS** habilita accesibilidad, separación de responsabilidades y despliegue reproducible.

---

## 14. Trabajo Futuro
- Implementar **comandos avanzados P2** (remove, edit, rename, copy/move, find, chown, chmod, unmount, add/delete en fdisk).  
- Formalizar **journal** (persistencia, replay de operaciones y UI de visualización).  
- Pruebas automatizadas de **consistencia de bitmaps** y **recuperación** (EXT3).

---

## 15. Anexos

### 15.1 Estructuras de Soporte
- **MountedPartition**, **User**, **Folder Entries**, etc.  
- Convenciones de **normalización de paths** y **nombres**.

### 15.2 Glosario
- **Inodo**, **Bloque**, **Superbloque**, **Bitmap**, **Journal**, **EXT2/EXT3**, **FF/BF/WF**.

---

> **GoDisk 2.0 integra teoría de sistemas de archivos con prácticas modernas de despliegue, ofreciendo una plataforma sólida para aprendizaje y evaluación académica.**
