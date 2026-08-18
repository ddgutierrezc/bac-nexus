# Ejecutar el spike de Catalogados en el equipo de trabajo BAC

Esta guía permite a un operador preparar, diagnosticar y ejecutar el spike actual `catalogspike` en un equipo Windows autorizado por BAC. El flujo es manual, acotado y orientado al PoC: no es un servicio de producción ni un servidor MCP desatendido.

Para el diseño técnico y los controles internos, consulte el [README técnico del spike](./README.md).

## Ruta rápida

- [ ] Confirme permisos, VPN/red, Go, Git, Code for IBM i y el JAR de Mapepire Server 2.3.5.
- [ ] Clone el repositorio o actualice una copia existente con `pull --ff-only`.
- [ ] Ejecute las pruebas y compile `catalogspike.exe`; si Windows Application Control bloquea la ubicación, compile únicamente en una ruta temporal aprobada.
- [ ] Ejecute `catalogspike setup`; prefiera una huella SSH verificada por un canal independiente y permita que el asistente detecte y verifique el JAR instalado.
- [ ] Ejecute el diagnóstico `offline` y confirme `"status":"offline-diagnostic"` y `"artifactVerified":true`.
- [ ] Ejecute `live` sin `-show-source` durante la primera prueba autorizada.
- [ ] Registre solo clasificación, cantidad de candidatos, tiempos, límites y evidencia de limpieza; no copie secretos ni código fuente.

## 1. Prerrequisitos y condiciones de inicio

No se presupone acceso de administrador. Si una instalación o una excepción de política lo requiere, solicítela mediante el proceso corporativo; no intente eludir el control.

| Requisito | Comprobación |
|---|---|
| Git | `git --version` |
| Go compatible | `go version`; el módulo declara **Go 1.23.0** en `go.mod` |
| Acceso BAC | VPN/red autorizada hacia el host IBM i para `setup` con `inspect` y para `live` |
| Política de ejecución | Permiso corporativo para ejecutar el binario compilado desde una ubicación aprobada |
| Code for IBM i | Extensión instalada y autorizada en el equipo de trabajo |
| Mapepire | JAR de Mapepire Server **2.3.5** disponible localmente; Nexus no lo descarga ni lo incluye |
| Permisos IBM i | Identidad de lectura autorizada para Catalogados, bibliotecas y miembros seleccionados |
| Aprobaciones | Host, identidad, carga/ejecución del JAR GPL-3.0, ventana de prueba y exposición de metadatos o fuente |

> **DETÉNGASE** si falta una aprobación, si el host no es inequívoco, si el JAR no coincide, si la política bloquea el binario o si la única forma de conectar exige debilitar SSH.

## 2. Obtener o actualizar el repositorio

Repositorio actual:

```text
https://github.com/ddgutierrezc/bac-nexus.git
```

### Primera copia

En PowerShell, desde una carpeta de trabajo aprobada:

```powershell
git clone https://github.com/ddgutierrezc/bac-nexus.git
Set-Location -LiteralPath .\bac-nexus
```

### Copia existente

Sustituya la ruta por la ubicación autorizada de su copia:

```powershell
git -C 'C:\ruta\aprobada\bac-nexus' pull --ff-only
Set-Location -LiteralPath 'C:\ruta\aprobada\bac-nexus'
```

`pull --ff-only` evita crear una integración local implícita. Si Git informa cambios locales o divergencia, deténgase y solicite revisión antes de modificar o descartar archivos.

## 3. Probar y compilar

Desde la raíz del repositorio:

```powershell
$env:GOPROXY = 'off'
go test -count=1 ./...
go vet ./...
go build -o .\catalogspike.exe ./cmd/catalogspike
$CatalogSpike = (Resolve-Path -LiteralPath .\catalogspike.exe).Path
```

`GOPROXY=off` impide que estos comandos descarguen módulos. Si las dependencias no están en la caché local de Go, la compilación fallará y deberá seguirse el proceso corporativo aprobado para obtenerlas.

### Alternativa para Windows Application Control

No deshabilite Device Guard, AppLocker, Smart App Control ni ninguna política corporativa. Si la política permite ejecutar únicamente desde una carpeta temporal aprobada, use una ruta suministrada por soporte o seguridad:

```powershell
$ApprovedTemp = 'C:\ruta\temporal\aprobada'
if (-not (Test-Path -LiteralPath $ApprovedTemp)) { throw 'La ruta temporal aprobada no existe' }
go build -o (Join-Path $ApprovedTemp 'catalogspike.exe') ./cmd/catalogspike
$CatalogSpike = Join-Path $ApprovedTemp 'catalogspike.exe'
```

La variable `$ApprovedTemp` es un marcador: `%TEMP%` no debe asumirse como autorizado. Si el binario sigue bloqueado, deténgase y entregue a soporte la ruta, la hora y el identificador del evento de política, sin pedir una desactivación.

## 4. Detección segura del JAR de Mapepire 2.3.5

La ruta rápida no requiere localizar el JAR manualmente. Después de la confianza de clave SSH, el usuario y el Java home opcional, `setup` inspecciona en modo de solo lectura únicamente la raíz estándar `.vscode/extensions` del usuario actual. Acepta solo el nombre de directorio exacto y sensible a mayúsculas `halcyontechltd.code-for-ibmi-3.0.12`, sin sufijos, y solo esta ruta relativa:

```text
dist/mapepire-server-2.3.5.jar
```

Antes de recorrer, cada componente existente de la raíz de extensiones y del directorio coincidente debe pasar `os.Lstat` sin `ModeSymlink`; un enlace se rechaza y no se resuelve para aceptarlo. Selecciona automáticamente el archivo solo si el candidato canónico es regular, no es enlace, no supera 64 MiB y coincide con esta huella SHA-256:

```text
41b1cfa67778ac204426f1dda0b51bd3f45fe3b89c91121d968660140acc0876
```

El asistente no recorre el directorio personal, el repositorio, perfiles, bóvedas ni carpetas temporales. Las instalaciones con sufijo de plataforma objetivo, VS Code Insiders o una ubicación personalizada no forman parte de la detección automática y usan la ruta manual absoluta. Tampoco descarga, descomprime, copia, incorpora ni licencia el JAR, y no requiere privilegios de administrador. Mapepire Server 2.3.5 sigue siendo un artefacto GPL-3.0 suministrado por separado mediante la instalación aprobada de Code for IBM i.

### Diagnóstico manual cuando la detección no es única

Confirme que la extensión esperada está instalada:

```powershell
code --list-extensions --show-versions |
  Select-String '^halcyontechltd\.code-for-ibmi@3\.0\.12$'
```

Revise únicamente los directorios esperados y la ruta relativa exacta, sin búsqueda recursiva:

```powershell
$ExtensionsRoot = Join-Path $env:USERPROFILE '.vscode\extensions'
if (-not (Test-Path -LiteralPath $ExtensionsRoot)) { throw 'No existe la carpeta local de extensiones de VS Code' }

$MapepireCandidates = Get-ChildItem -LiteralPath $ExtensionsRoot -Directory |
  Where-Object { $_.Name -ceq 'halcyontechltd.code-for-ibmi-3.0.12' } |
  ForEach-Object {
    $Candidate = Join-Path $_.FullName 'dist\mapepire-server-2.3.5.jar'
    if (Test-Path -LiteralPath $Candidate -PathType Leaf) { Get-Item -LiteralPath $Candidate }
  }

$MapepireCandidates | Select-Object FullName, Length
```

Ejemplo representativo; la ruta real depende del usuario:

```text
FullName
--------
C:\Users\<USUARIO>\.vscode\extensions\halcyontechltd.code-for-ibmi-3.0.12\dist\mapepire-server-2.3.5.jar
```

Si aparece exactamente un candidato, asígnelo y calcule su huella:

```powershell
if (@($MapepireCandidates).Count -ne 1) { throw 'Se esperaba exactamente un JAR candidato; revise la instalación' }
$MapepireJar = $MapepireCandidates[0].FullName
Get-FileHash -Algorithm SHA256 -LiteralPath $MapepireJar
```

Ejemplo de salida, sin afirmar una ruta real:

```text
Algorithm Hash                                                             Path
--------- ----                                                             ----
SHA256    41B1CFA67778AC204426F1DDA0B51BD3F45FE3B89C91121D968660140ACC0876 C:\...\<MAPEPIRE>.jar
```

Si no hay un candidato verificado o si la ubicación exacta falla por tipo, lectura, enlace o huella, `setup` informa una razón sanitizada y solicita `Local Mapepire Server 2.3.5 JAR path:`; proporcione una ruta absoluta. Use también esa alternativa para sufijos de plataforma objetivo, VS Code Insiders y ubicaciones personalizadas. La comprobación manual ayuda a resolver la instalación, pero no sustituye el control del programa: `setup`, `offline` con `-mapepire-jar` y cada ejecución `live` vuelven a abrir y verificar el JAR. Un archivo cambiado, reemplazado, enlazado o con otra huella falla de forma cerrada antes de solicitar secretos o persistir datos.

La detección portable de enlaces se limita a puntos de reanálisis que el runtime de Go expone mediante `os.Lstat` como `ModeSymlink`. Algunos tipos exóticos de punto de reanálisis de Windows podrían no aparecer con ese modo; esta es una limitación restante del sistema operativo/runtime. La hoja JAR candidata conserva la canonicalización y la verificación basada en descriptor.

### Por qué ya no se requiere `ssh-keyscan`

En el equipo Windows observado, `ssh-keyscan` falla al negociar `sntrup761x25519-sha512@openssh.com`. Esto no demuestra que IBM i necesite algoritmos SSH heredados ni SHA-1. El spike ya no depende de `ssh-keyscan`: el modo `inspect` realiza un intercambio SSH acotado con los algoritmos seguros soportados por la dependencia Go, captura la clave durante el intercambio y se detiene antes de autenticar.

No habilite algoritmos KEX heredados de forma global para corregir este síntoma.

## 5. Configuración interactiva

Ejecute el asistente desde una terminal real:

```powershell
& $CatalogSpike setup
```

Los campos no secretos se recortan; las contraseñas no se muestran. El orden actual de los mensajes es:

```text
Host-key inspection is optional first-contact discovery, not independent server identity verification.
Connection name: <PERFIL>
Host: <HOST_IBM_I>
Port [22]: <ENTER_O_PUERTO>
Host-key enrollment [manual/inspect] (manual recommended; inspect is spike-only TOFU fallback): <manual|inspect>
```

### Ruta recomendada: huella verificada manualmente

Use `manual` cuando un canal corporativo independiente proporcione la huella esperada:

```text
Independently verified SHA256 host-key fingerprint: SHA256:<HUELLA_VERIFICADA>
Username: <USUARIO_IBM_I>
Optional Java home: <ENTER_PARA_VALOR_PREDETERMINADO_O_RUTA_APROBADA>
Mapepire Server 2.3.5 was automatically found and verified.
IBM i password for <PERFIL>: <ENTRADA_OCULTA>
Vault master passphrase for <PERFIL>: <ENTRADA_OCULTA>
Confirm vault master passphrase for <PERFIL>: <ENTRADA_OCULTA>
Create this profile and encrypted vault? [yes/no]: yes
```

Esta ruta guarda `hostKeyTrust: "verified"`. La aprobación final acepta `yes` sin distinguir mayúsculas, pero se recomienda escribir `yes` exactamente como se muestra.

Si no aparece el mensaje de detección automática, el asistente explica que no encontró un candidato verificado único y solicita `Local Mapepire Server 2.3.5 JAR path:`. Introduzca la ruta absoluta obtenida con el diagnóstico de la sección 4. Una ruta inválida termina `setup` antes de las solicitudes de contraseña y frase maestra.

### Alternativa exclusiva del spike: inspección y TOFU

Si todavía no existe una huella independiente y la prueba TOFU fue autorizada, escriba `inspect`. El programa realiza una conexión de inspección con límite de 15 segundos y sin enviar la contraseña de IBM i. Después muestra, por `stderr`, el algoritmo y la huella observados:

```text
Observed SSH host key algorithm <ALGORITMO> with fingerprint SHA256:<HUELLA_OBSERVADA>
WARNING: this key came from the current connection and is not independently verified. Production enrollment requires an approved independent channel.
Trust this observed key for this spike? Type exact yes: yes
```

En esta confirmación debe escribir **exactamente** `yes` en minúsculas, sin espacios ni tabulaciones antes o después. `YES`, ` yes`, `yes `, una línea vacía o cualquier otro valor abortan antes de solicitar credenciales y no crean perfil ni bóveda.

Esta ruta guarda `hostKeyTrust: "tofu"`: documenta que la confianza procede de la conexión actual y que no hubo verificación independiente. `live` nunca vuelve a inspeccionar ni acepta otra clave; siempre exige la huella fijada. Si la clave del host cambia, la ejecución falla con `SSH host-key fingerprint mismatch`; no actualice la huella hasta que el cambio sea validado por un canal aprobado.

### Persistencia y recuperación

En Windows, el código usa `os.UserConfigDir()` y estos patrones:

```text
%AppData%\BAC Nexus\profiles\<PERFIL>.json
%AppData%\BAC Nexus\credentials\<PERFIL>.vault
```

El perfil contiene metadatos no secretos: host, puerto, usuario, huella, procedencia de confianza, ruta local del JAR, Java home y modo de credencial. La bóveda cifra la contraseña de IBM i con Argon2id y AES-256-GCM, vinculada al nombre del perfil. La frase maestra no se almacena, se solicita en cada `live` y **no puede recuperarse** si se pierde.

El asistente crea primero la bóveda y publica el perfil al final. Si falla la publicación del perfil e igualmente falla la limpieza de la bóveda, el mensaje identifica un huérfano recuperable por nombre. No repita `setup` sin consultar primero:

```powershell
& $CatalogSpike credentials status -profile '<PERFIL>'
& $CatalogSpike credentials delete -profile '<PERFIL>'
```

## 6. Administrar credenciales

### Estado

```powershell
& $CatalogSpike credentials status -profile '<PERFIL>'
```

Resultados posibles:

```json
{"exists":true}
```

```json
{"exists":false}
```

El estado no descifra la bóveda ni enumera otros perfiles.

### Crear

```powershell
& $CatalogSpike credentials set -profile '<PERFIL>'
```

Solicita de forma oculta la contraseña de IBM i y la frase maestra. Si ya existe una bóveda, falla sin reemplazarla. En caso de éxito:

```json
{"status":"stored","profile":"<PERFIL>","cleanupPending":false}
```

### Reemplazar o rotar

```powershell
& $CatalogSpike credentials set -profile '<PERFIL>' -replace
```

`-replace` requiere que la bóveda exista. La rotación conserva una ruta de reversión durante la publicación. Si la nueva bóveda quedó comprometida pero sigue pendiente eliminar el respaldo, el JSON informa `"cleanupPending":true` y `stderr` informa que la limpieza se reintentará en la siguiente operación. Consulte el estado; no repita la mutación automáticamente.

### Eliminar

```powershell
& $CatalogSpike credentials delete -profile '<PERFIL>'
```

La eliminación es idempotente:

```json
{"deleted":true}
```

indica que eliminó la bóveda; `{"deleted":false}` indica que no existía. El perfil no se elimina. Un perfil en modo `vault` sin bóveda falla de forma cerrada en `live`; recupérelo con `credentials set` o retire el perfil mediante un procedimiento manual aprobado.

## 7. Diagnóstico completamente fuera de línea

Este comando construye la consulta acotada, verifica el JAR local y no abre conexiones de red:

```powershell
& $CatalogSpike offline -item 'ABC$DEF' -production-library '<BIBLIOTECA_PRODUCCION>' -mapepire-jar $MapepireJar
```

PowerShell expande `$...` dentro de comillas dobles. Para nombres que contienen `$`, use **comillas simples**, como `'ABC$DEF'`.

Confirme en el JSON:

- `"status":"offline-diagnostic"`;
- `"query":{"statement":"catalogados.search.v1",...}` sin SQL ni parámetros;
- `"rowCap":51`;
- `"mapepireVersion":"2.3.5"`;
- la huella esperada;
- `"artifactVerified":true`.

Sin `-mapepire-jar`, el diagnóstico sigue siendo fuera de línea, pero informa `"artifactVerified":false`.

## 8. Primera ejecución en vivo autorizada

El siguiente ejemplo conserva literalmente el `$` del ítem y mantiene la fuente oculta:

```powershell
& $CatalogSpike live -profile '<PERFIL>' -item 'ABC$DEF' -production-library '<BIBLIOTECA_PRODUCCION>' -max-bytes 1048576 -max-lines 10000
```

El JAR se toma del perfil. Puede proporcionar una sustitución explícita con `-mapepire-jar $MapepireJar`. La operación completa tiene un límite de 60 segundos.

Los límites predeterminados son 1 MiB y 10 000 líneas. Los máximos absolutos aceptados son 4 MiB (`4194304`) y 50 000 líneas:

```powershell
& $CatalogSpike live -profile '<PERFIL>' -item 'ABC$DEF' -max-bytes 4194304 -max-lines 50000
```

La salida predeterminada suprime el contenido fuente. Solo después de aprobar su destino y exposición, habilítelo explícitamente:

```powershell
& $CatalogSpike live -profile '<PERFIL>' -item 'ABC$DEF' -show-source -max-bytes 1048576 -max-lines 10000
```

`-show-source` escribe una advertencia y agrega `source.content` al JSON. Ese contenido es sensible; redirigir la salida crea otra copia fuera del control del spike.

### Clasificaciones esperadas

| `classification` | Significado y siguiente acción |
|---|---|
| `not-found` | La consulta no devolvió candidatos; verifique el ítem y la biblioteca, sin forzar una lectura. |
| `not-exact` | Solo apareció una coincidencia parcial; el spike se detiene antes de leer fuente. |
| `ambiguous` | Hay varios candidatos; seleccione uno usando las cuatro coordenadas devueltas. |
| `selected` | Se eligió un candidato exacto y se intentó recuperar el miembro acotado. |

Con `selected`, revise `source.remoteSize`, `bytes`, `lines`, `truncated` y `cleanup`. Un éxito completo debe informar `cleanup: "removed"`. La consulta acepta como máximo 50 candidatos; el candidato 51 provoca un error de límite.

### Resolver una ambigüedad

Copie exactamente las cuatro coordenadas de **un mismo candidato** devuelto por la consulta:

```powershell
& $CatalogSpike live -profile '<PERFIL>' -item '<ITEM>' `
  -selector-library '<SOURCE_LIBRARY>' `
  -selector-file-base '<SOURCE_FILE_BASE>' `
  -selector-object-type '<OBJECT_TYPE>' `
  -selector-source-type '<SOURCE_TYPE>'
```

Las cuatro banderas son obligatorias en conjunto. Un selector parcial falla; uno completo debe coincidir exactamente con un candidato de esa misma búsqueda. El spike no adivina.

## 9. Evidencia segura de una ejecución exitosa

Registre únicamente:

- fecha y ventana autorizada;
- versión o commit probado, sin credenciales;
- clasificación y `candidateCount`;
- coordenadas no sensibles aprobadas del candidato seleccionado;
- duración aproximada y si ocurrió un timeout;
- límites solicitados, `bytes`, `lines` y `truncated`;
- resultado `cleanup` del temporal fuente;
- resultado de la verificación SHA-256 del JAR;
- SQLSTATE y código SQL tipados, si existieron, sin copiar mensajes remotos adicionales.

No capture pantalla mientras se escriben secretos. No adjunte el perfil completo si sus metadatos de infraestructura no están autorizados para compartirse. No adjunte `source.content`.

## 10. Solución de problemas

La tabla usa resultados y mensajes que el código actual produce; otros mensajes del sistema operativo, Git, Java o SSH deben reportarse textualmente y sanitizados, no reinterpretarse.

| Situación | Evidencia actual | Acción segura |
|---|---|---|
| Device Guard o Windows Application Control | El sistema operativo bloquea la creación o ejecución; el texto depende de la política corporativa. | Pruebe solo la ruta temporal aprobada. Si continúa, deténgase y reporte ruta, hora, hash del binario e identificador de evento. Nunca deshabilite la política. |
| JAR incorrecto o alterado | `setup` informa `Mapepire Server JAR verification failed` sin revelar la ruta; `offline` puede informar `Mapepire Server JAR checksum mismatch`; después de carga puede aparecer `uploaded Mapepire JAR checksum mismatch`. | Vuelva a localizar el JAR 2.3.5. No cambie la huella esperada ni use otro JAR. |
| Detección automática sin resultado único | El asistente informa cero candidatos verificados, candidatos rechazados o la cantidad de candidatos verificados ambiguos, sin imprimir sus rutas. | Confirme Code for IBM i 3.0.12, use únicamente el diagnóstico acotado de la sección 4 y proporcione una ruta absoluta. No elija por fecha u orden. |
| `ssh-keyscan` falla con `sntrup...` | Fallo conocido del cliente local al negociar KEX. | No se requiere `ssh-keyscan`. Use huella manual verificada o el `inspect` TOFU autorizado. No habilite KEX heredado globalmente. |
| `inspect` no negocia | `SSH host-key inspection found no mutually supported ... algorithm; weak algorithms will not be enabled`. | Deténgase; conserve la clase de algoritmo reportada y solicite revisión de compatibilidad. |
| Clave del host cambió | `SSH host-key fingerprint mismatch`. | Deténgase. Verifique el cambio por un canal independiente; no desactive la verificación ni edite la huella por ensayo. |
| Falta la bóveda | `vault-mode profile has no credential vault; run credentials set explicitly`. | Ejecute `credentials status` y, si corresponde, `credentials set`. No espere una degradación automática a contraseña interactiva. |
| Frase maestra incorrecta o bóveda alterada | `credential vault authentication failed`. | Reintente cuidadosamente. Si la frase se perdió, no hay recuperación: cree una nueva bóveda con la contraseña autorizada. |
| Bóveda huérfana tras `setup` | El mensaje indica que el perfil no fue publicado y dirige a `credentials status/delete -profile "<PERFIL>"`. | Consulte estado y elimine el huérfano antes de repetir la configuración. |
| Java o Mapepire no inicia | `Mapepire launch failed: ...`, ausencia del identificador de job o error de protocolo tipado. | Confirme Java home aprobado, ejecución del JAR y permisos. Reporte el error sanitizado; no pruebe comandos Java/SSH arbitrarios. |
| Sin autorización SQL | `Mapepire SQL request failed (SQLSTATE <...>, SQL code <...>)`. | Capture solo SQLSTATE y código. Solicite revisión de autoridad; no amplíe permisos por cuenta propia. |
| No encontrado | JSON con `classification: "not-found"` o `"not-exact"`. | Revise ítem y biblioteca. No trate una coincidencia parcial como exacta. |
| Ambiguo | JSON con `classification: "ambiguous"`. | Use las cuatro banderas `-selector-*` con las coordenadas de un candidato devuelto. |
| Timeout | `SSH host-key inspection timed out` durante `inspect`; en `live`, la operación respeta el límite de 60 segundos y puede terminar por contexto. | Registre fase y duración, confirme red/VPN y deténgase si el problema persiste. No quite los límites. |
| Falló limpieza de fuente | El error incluye `remote source temporary-file cleanup failed`; `cleanup` pasa a `failed`. | Deténgase y solicite comprobar/eliminar el temporal Nexus autorizado. No continúe exponiendo fuente. |
| Limpieza de rotación pendiente | JSON con `"cleanupPending":true` y advertencia de reversión pendiente. | Consulte estado; la siguiente operación reintenta la limpieza. No repita a ciegas la rotación. |

## 11. Condiciones explícitas para detenerse

Detenga la operación y escale cuando ocurra cualquiera de estas condiciones:

- falta aprobación de host, usuario, bibliotecas, JAR, fuente o ventana de prueba;
- la huella SSH cambia o no puede verificarse según el nivel de confianza autorizado;
- se requiere deshabilitar verificación de host, habilitar algoritmos inseguros o ampliar permisos;
- el JAR no coincide con la huella fijada;
- Windows Application Control bloquea todas las ubicaciones aprobadas;
- el resultado supera límites, excede 50 candidatos o no puede seleccionarse sin adivinar;
- un timeout es repetible o la limpieza remota no puede demostrarse;
- una salida podría exponer fuente o metadatos a un destino no aprobado;
- el comportamiento real difiere del contrato documentado.

## 12. Lista de seguridad

- [ ] Nunca pegue, comparta, registre ni capture la contraseña de IBM i.
- [ ] Nunca pegue, comparta, registre ni capture la frase maestra.
- [ ] Nunca confirme TOFU sin comprender que no es verificación independiente.
- [ ] Nunca deshabilite la verificación de clave del host.
- [ ] Nunca habilite algoritmos KEX heredados de forma global.
- [ ] Nunca deshabilite Device Guard, AppLocker ni otra política corporativa.
- [ ] Nunca confirme cambios de huella sin un canal independiente aprobado.
- [ ] Nunca incorpore al repositorio perfiles, bóvedas, JAR, resultados vivos ni salida fuente.
- [ ] Nunca redirija `-show-source` a archivos o sistemas no aprobados.
- [ ] Nunca convierta este spike en una herramienta SSH, SQL o CL genérica.

## 13. Alcance y limitaciones del PoC

- Es un spike manual y acotado, no una solución de producción.
- Solo consulta Catalogados mediante una sentencia preparada fija y recupera un miembro seleccionado.
- Requiere intervención humana, aprobaciones y una terminal real para secretos.
- La frase maestra se solicita en cada ejecución `live`; todavía no existe ejecución MCP desatendida.
- No hay prueba automática contra BAC ni un entorno IBM i vivo.
- TOFU es una alternativa de primera conexión exclusiva del spike; no sustituye verificación independiente para producción.
- El límite total de `live` es 60 segundos y la selección/carga permanece acotada.
- No modifica objetos fuente de IBM i, pero carga el JAR aprobado y crea temporales propios que debe limpiar.

## 14. Ayuda y reporte sanitizado

Ayuda de nivel raíz:

```powershell
& $CatalogSpike help
& $CatalogSpike -h
```

Ayuda de subcomandos:

```powershell
& $CatalogSpike setup -h
& $CatalogSpike configure -h
& $CatalogSpike offline -h
& $CatalogSpike live -h
& $CatalogSpike credentials -h
& $CatalogSpike credentials set -h
& $CatalogSpike credentials status -h
& $CatalogSpike credentials delete -h
```

Para reportar resultados, envíe un resumen sanitizado con:

```text
Commit o versión: <VALOR>
Equipo/entorno: equipo BAC autorizado, sin nombre de host si no está aprobado
Fase: <build|setup-manual|setup-inspect|offline|live>
Clasificación: <not-found|not-exact|ambiguous|selected|no-aplica>
Cantidad de candidatos: <N>
Límites: bytes=<N>, líneas=<N>
Truncado: <true|false|no-aplica>
Limpieza: <removed|failed|not-created|no-aplica>
Duración aproximada: <VALOR>
SHA-256 del JAR coincide: <sí|no>
SQLSTATE/código SQL: <VALORES_TIPADOS_O_NO_APLICA>
Error sanitizado: <TEXTO_SIN_HOST_USUARIO_RUTAS_SECRETOS_NI_FUENTE>
```

Antes de enviarlo, elimine contraseñas, frase maestra, contenido fuente, SQL ejecutable, valores de parámetros y cualquier dato de infraestructura que no esté autorizado para compartirse.
