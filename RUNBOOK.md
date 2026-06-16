# Deployment & Operations Runbook (RUNBOOK)
## Project: FamilyHealth+ (Diaspora-First Family Care Platform)
**Version:** 1.0  
**Date:** June 16, 2026  
**Author:** AI Systems Architect  

---

This runbook outlines how to set up the development environment, perform database migrations, run local services, and deploy the FamilyHealth+ application in a production environment.

---

## 1. Prerequisites & Environment Setup

The codebase utilizes Nix flakes for automated dev environment provisioning. Ensure you have the following installed on your local machine:
*   **Git**
*   **Docker Desktop** (v20+)
*   **Nix** (with flakes enabled) or manual installation of **Go 1.18+** and **Node.js 18+ / Yarn**.

### Quick Start with Nix
Run the following at the root of the repository to enter the development shell with all dependencies (Go, Node, Yarn, SQLite) pre-loaded:
```bash
nix develop
```

---

## 2. Development Setup

### 2.1 Backend Build & Start
1.  **Download and Vendor Dependencies**:
    ```bash
    make dep-backend
    ```
2.  **Run Database Migrations**:
    Apply current database schema changes (GORM and custom FamilyHealth tables) to your local SQLite DB:
    ```bash
    go run backend/cmd/fasten/fasten.go migrate --config ./config.dev.yaml --debug
    ```
3.  **Start the Go API Server**:
    Starts the API backend listening on `http://localhost:8080` (or as configured in `config.dev.yaml`):
    ```bash
    go run backend/cmd/fasten/fasten.go start --config ./config.dev.yaml --debug
    ```

### 2.2 React Native (Mobile App) Setup
1.  Navigate to the mobile app directory (after migrating from Angular):
    ```bash
    cd mobile
    yarn install
    ```
2.  **Start Metro Bundler (Expo)**:
    ```bash
    npx expo start
    ```
3.  Press `i` for iOS Simulator or `a` for Android Emulator. Ensure the mobile client has access to `http://localhost:8080/api` (use local IP for physical devices, e.g., `http://192.168.1.50:8080/api`).

---

## 3. Database Operations

FamilyHealth+ uses GORM migrations. When extending schemas (e.g., adding the family linkages or telemetry tables):

1.  Add GORM model struct definitions in `backend/pkg/models/`.
2.  Register the model in `backend/pkg/database/gorm_repository_migrations.go` inside the migration slice:
    ```go
    // Example GORM AutoMigrate registration
    db.AutoMigrate(&models.FamilyLink{}, &models.VitalTelemetry{}, &models.EmergencyQR{})
    ```
3.  Run the migrate command (see step 2 in section 2.1).

---

## 4. Production Deployment

Production deployments are containerized via Docker and support clustering or single-node VPS hosting.

### 4.1 Production Configurations
Create a production configuration file `config.prod.yaml` at the root:
```yaml
database:
  driver: postgres # Use postgres for production
  connection: "host=db.tfoco.local user=fh_user password=secure_pw dbname=familyhealth port=5432 sslmode=require"
  encryption:
    key: "YOUR_SECURE_32_BYTE_HEX_ENCRYPTION_KEY"
log:
  level: "INFO"
  format: "json"
auth:
  jwt_secret: "YOUR_JWT_SECRET"
```

### 4.2 Single-Node Docker Deployment
Use the production Compose configuration to launch the PostgreSQL database and Go backend:
```bash
docker-compose -f docker-compose-prod.yml up -d
```

---

## 5. Security & Regulatory Compliance Checklists

### 5.1 GDPR Compliance (EU/Portugal)
*   **Data Portability**: Users can export their complete health record (including FHIR observations and telemetry) via `/api/v1/patient/export` in standard FHIR JSON format.
*   **Right to Erasure**: Deleting a User account triggers a cascading database delete removing `FamilyLinks`, `VitalTelemetry`, `SourceCredentials`, and `EmergencyQR` records.

### 5.2 DPDP Compliance (India)
*   **Consent Log**: Log all consent agreements when connecting a parent's record to a sponsor's dashboard in a secure audit log table.
*   **Language Support**: Ensure user agreements and vitals logging interfaces support English, Hindi, and regional languages.

---

## 6. Accelerating Development Workflows

Run these commands to speed up your build and coding workflows:

### 6.1 Syncing Type Definitions (Tygo)
Whenever you update domain models in Go (`backend/domain/model/*.go`), automatically compile them into TypeScript types for the frontend:
```bash
make generate-backend
```
This runs the `tygo generate` command under the hood, updating type files inside your frontend components.

### 6.2 Spin Up Dev Environment via Devcontainer
If you have VSCode or any Devcontainer-compatible IDE:
1. Open the project root.
2. Select **"Reopen in Container"** when prompted.
3. This boots a virtual environment with pre-installed tools, Go environment, local Postgres, and Yarn packages.

### 6.3 Deploying AGY SDK Autonomous Agents
For automated page construction and testing, activate the Antigravity multi-agent systems using the SDK:
```bash
# Initialize and launch autonomous coding agents in the workspace
npx agy launch --agent UI-UX-Builder --prompt "Design a NativeWind-based parent vitals dashboard screen."
```
