# 🚀 Local Development - Quick Start

Este archivo te guía para correr el proyecto localmente en tu máquina.

## ✅ Todo Listo

### Lo que hicimos
- ✅ Reorganizamos archivos (docs/, scripts/)
- ✅ Limpiamos ambientes Docker
- ✅ Tests E2E al 90% (44/49 passing)
- ✅ Pusheado a GitHub

### Estado del Proyecto
```
📊 E2E Tests: 44/49 passing (90%)
   - Auth: 5/5 (100%)
   - Dashboard Widget: 10/10 (100%)
   - Detection: 7/7 (100%)
   - Secrets: 11/11 (100%)
   - Resolution: 7/8 (87.5%)
   - Comparison: 5/8 (62.5%)
```

## 🏃 Cómo Correr Localmente

### 1. Iniciar Servicios

```bash
# Desde la raíz del proyecto
cd /home/nk/secret-manager

# Iniciar todos los servicios (backend + frontend + postgres)
docker compose up -d

# Ver logs
docker compose logs -f
```

### 2. Verificar que Todo Funciona

```bash
# Backend health check
curl http://localhost:8080/health

# Frontend
# Abrí http://localhost:3000 en tu browser
```

### 3. Login

1. Ir a http://localhost:3000
2. Click "Login"
3. El mock OAuth te redirige automáticamente
4. Deberías ver el dashboard

**Usuarios de prueba:**
- `dev@example.com` - Developer (editor en development namespace)
- `admin@example.com` - Admin (admin en todos los namespaces)

### 4. Explorar Features

```bash
# Ver secrets
http://localhost:3000/secrets

# Ver drift detection
http://localhost:3000/drift

# Ver dashboard
http://localhost:3000/dashboard
```

## 🧪 Correr Tests

### Tests E2E

```bash
# Desde frontend/
cd frontend

# Levantar ambiente de prueba
docker compose -f ../docker-compose.e2e.yml up -d

# Correr tests
docker run --rm \
  --network=secret-manager_secretmanager-e2e \
  -v $(pwd):/app -w /app \
  -e PLAYWRIGHT_BASE_URL=http://frontend:3000 \
  -e NEXT_PUBLIC_API_URL=http://backend:8080 \
  mcr.microsoft.com/playwright:v1.58.2-noble \
  npx playwright test --reporter=list

# Bajar ambiente
docker compose -f ../docker-compose.e2e.yml down -v
```

### Tests Rápidos (Individual Suite)

```bash
# Solo auth tests
npx playwright test auth/

# Solo secrets tests
npx playwright test secrets/

# Solo drift tests
npx playwright test drift/
```

## 🛠️ Comandos Útiles

### Backend

```bash
# Logs del backend
docker compose logs -f backend

# Restart backend
docker compose restart backend

# Rebuild backend
docker compose build backend
docker compose up -d backend
```

### Frontend

```bash
# Logs del frontend
docker compose logs -f frontend

# Restart frontend
docker compose restart frontend

# Correr frontend fuera de Docker (más rápido para dev)
cd frontend
npm run dev
# Abre http://localhost:3000
```

### Base de Datos

```bash
# Conectar a PostgreSQL
docker compose exec postgres psql -U dev -d secretmanager

# Ver datos
\dt  # ver tablas
SELECT * FROM users;
SELECT * FROM namespaces;
SELECT * FROM secrets;
```

## 🧹 Limpiar Todo

```bash
# Parar y eliminar contenedores + volumes
docker compose down -v

# Limpiar sistema Docker completo
docker system prune -af

# Limpiar volumes huérfanos
docker volume prune -f
```

## 📁 Estructura del Proyecto

```
secret-manager/
├── docs/                    # 📖 Documentación
│   ├── README.md           # Índice de docs
│   ├── setup/              # Guías de setup
│   ├── architecture/       # Arquitectura técnica
│   └── testing/            # Docs de testing
├── scripts/                # 🔨 Scripts útiles
│   └── dev/               # Scripts de desarrollo
├── backend/                # 🔧 API Go
│   ├── cmd/server/        # Entry point
│   ├── internal/          # Código interno
│   └── Makefile           # Comandos backend
├── frontend/               # 🎨 Next.js UI
│   ├── app/               # Pages (App Router)
│   ├── components/        # React components
│   └── e2e/               # Tests E2E
├── docker-compose.yml      # Dev environment
└── docker-compose.e2e.yml  # E2E test environment
```

## 🐛 Troubleshooting

### Puerto 3000 ya en uso
```bash
# Encontrar qué proceso usa el puerto
lsof -ti:3000

# Matar el proceso
kill -9 $(lsof -ti:3000)
```

### PostgreSQL no conecta
```bash
# Ver si está corriendo
docker compose ps postgres

# Ver logs
docker compose logs postgres

# Recrear
docker compose down -v
docker compose up -d postgres
```

### Frontend no carga
```bash
# Verificar logs
docker compose logs frontend

# Rebuil frontend
docker compose build frontend --no-cache
docker compose up -d frontend
```

## 📚 Documentación

- **Docs completas**: `docs/README.md`
- **Quickstart**: `docs/setup/quickstart.md`
- **Testing**: `docs/testing/`
- **Architecture**: `docs/architecture/`

## 🎯 Próximos Pasos

1. **Explorar la app** - Probá crear secrets, ver drift, etc.
2. **Revisar código** - Familiarizate con backend/frontend
3. **Pensar features** - Qué querés agregar/mejorar
4. **Pedime cambios** - Cuando sepas qué querés, decime y lo implementamos

---

**¿Listo para empezar?** 🚀

```bash
docker compose up -d
# Abrí http://localhost:3000
```
