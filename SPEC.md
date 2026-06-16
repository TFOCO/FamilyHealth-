# Technical Specification (SPEC)
## Project: FamilyHealth+ (Diaspora-First Family Care Platform)
**Version:** 1.0  
**Date:** June 16, 2026  
**Author:** AI Systems Architect  

---

## 1. System Architecture

FamilyHealth+ is designed with a decentralized, hybrid-cloud architecture. It builds on the open-source Fasten Health FHIR aggregator, extending its Go backend and replacing its Angular web UI with a cross-platform mobile application.

```mermaid
graph TD
    subgraph Mobile Apps (React Native)
        S_App[Sponsor App - USA/Europe]
        P_App[Parent App - India/Portugal]
    end

    subgraph Backend Services (Go API)
        Auth[Auth & OAuth2 Gateway]
        FHIR_Eng[FHIR Aggregator Engine]
        AI_Eng[AI Analytics Engine]
        Pay_Eng[Escrow Payment Engine]
        QR_Eng[Emergency QR Resolver]
    end

    subgraph Database Layer
        Local_DB[(SQLCipher local encryption)]
        Cloud_DB[(PostgreSQL Database)]
    end

    subgraph External Systems
        EHR[US EHRs: Epic, Cerner]
        ABDM[India ABDM Gateway]
        SNS24[Portugal SNS24 Adapter]
        LLM[OpenAI / LLM API]
        SMS[Twilio / WhatsApp API]
    end

    S_App -->|REST API| Backend Services
    P_App -->|REST API| Backend Services
    
    Backend Services --> Cloud_DB
    P_App -->|Local Cache| Local_DB
    
    FHIR_Eng --> EHR
    FHIR_Eng --> ABDM
    FHIR_Eng --> SNS24
    
    AI_Eng --> LLM
    QR_Eng --> SMS
```

---

## 2. Database Schema Extensions (Go/GORM)

We will extend Fasten's existing GORM models to support family linkages, remote sponsor roles, vital sign telemetry, and cross-border transactions.

### 2.1 Family Relationship Model
Stores the links between Sponsors (the diaspora payers) and Subjects (the parents/family members).

```go
type FamilyLink struct {
	ModelBase
	SponsorID  uint         `gorm:"index;not null" json:"sponsor_id"`
	Sponsor    User         `gorm:"foreignKey:SponsorID"`
	SubjectID  uint         `gorm:"index;not null" json:"subject_id"`
	Subject    User         `gorm:"foreignKey:SubjectID"`
	Relation   string       `gorm:"size:50;not null" json:"relation"` // "Father", "Mother", "Spouse", etc.
	AccessRole pkg.UserRole `gorm:"size:20;not null" json:"access_role"` // "admin", "viewer", "billing"
}
```

### 2.2 Vital Telemetry Model
Optimized table for time-series recording of vital signs (BP, glucose, heart rate) for dashboard rendering and AI analysis.

```go
type VitalTelemetry struct {
	ModelBase
	SubjectID   uint      `gorm:"index;not null" json:"subject_id"`
	VitalType   string    `gorm:"size:50;index;not null" json:"vital_type"` // "blood_pressure", "blood_glucose", "heart_rate"
	ValueMetric float64   `gorm:"type:numeric(10,2);not null" json:"value_metric"`
	ValueUnit   string    `gorm:"size:20;not null" json:"value_unit"` // "mmHg", "mg/dL", "bpm"
	ContextData string    `gorm:"type:text" json:"context_data"` // JSON holding secondary values (e.g. Diastolic vs Systolic)
	RecordedAt  time.Time `gorm:"index;not null" json:"recorded_at"`
}
```

### 2.3 Emergency QR Mapping Model
Links a parent’s physical QR code payload to their user profile and critical emergency details.

```go
type EmergencyQR struct {
	ModelBase
	SubjectID      uint   `gorm:"index;unique;not null" json:"subject_id"`
	QRHash         string `gorm:"size:64;unique;index;not null" json:"qr_hash"`
	BloodGroup     string `gorm:"size:5" json:"blood_group"`
	Allergies      string `gorm:"type:text" json:"allergies"`
	ActiveMeds     string `gorm:"type:text" json:"active_meds"`
	SponsorPhone   string `gorm:"size:20;not null" json:"sponsor_phone"`
	IsActive       bool   `gorm:"default:true" json:"is_active"`
}
```

---

## 3. API Design

### 3.1 AI Clinical Summarizer
Generates a structured, patient-friendly summary from raw FHIR records.
*   **Endpoint**: `POST /api/v1/family/summary`
*   **Headers**: `Authorization: Bearer <token>`
*   **Request Body**:
    ```json
    {
      "subject_id": 45,
      "resource_ids": ["fhir-obs-12", "fhir-med-4"]
    }
    ```
*   **Response**:
    ```json
    {
      "status": "success",
      "summary": "Your father's blood sugar has stabilized at 110 mg/dL. The doctor added a daily dosage of Metformin. Monitor him for nausea or changes in appetite.",
      "red_flags": ["Ask if he experiences dizziness in the morning", "Ensure he takes Metformin *after* breakfast"]
    }
    ```

### 3.2 Emergency QR Resolve
Endpoint scanned by emergency responders to pull critical data and notify the sponsor.
*   **Endpoint**: `GET /api/v1/emergency/qr/:qr_hash`
*   **Authentication**: None (Open access for paramedics/neighbors).
*   **Response**:
    ```json
    {
      "blood_group": "O-Positive",
      "critical_allergies": ["Penicillin", "Peanuts"],
      "medications": [
        {"name": "Metformin", "dosage": "500mg daily"},
        {"name": "Amlodipine", "dosage": "5mg daily"}
      ],
      "emergency_contacts": [
        {"name": "Rajesh (Sponsor)", "phone": "+1-555-0199", "relation": "Son"}
      ]
    }
    ```
*   **Internal Action**: Triggers async worker to send WhatsApp/SMS alert to `SponsorPhone` with geographical coordinates.

---

## 4. React Native Migration Plan

To make the app accessible for parents in India and sponsors globally, the frontend web app (Angular v14) will be replaced by a cross-platform **React Native (Expo)** mobile application.

### 4.1 Native Mobile Stack
*   **Framework**: Expo (React Native v0.72+) with TypeScript.
*   **State Management**: Redux Toolkit for offline cache, query results, and token storage.
*   **Styling**: NativeWind (Tailwind CSS for React Native) for rich visual components.
*   **Local Storage**: `expo-sqlite` backed by SQLCipher for encrypted on-device health record aggregation.

### 4.2 Auth & Tokens handling
*   Use `expo-secure-store` to encrypt and store OAuth2 access tokens and refresh tokens.
*   Background sync runs using `expo-background-fetch` to query EHR systems hourly and keep vitals updated in the background without user intervention.

---

## 5. Regional Gateways & Integration Specs

### 5.1 India: ABDM Gateway (Ayushman Bharat Digital Mission)
*   **Standard**: MCTS/ABDM APIs.
*   **Auth**: Aadhaar-based verification creating ABHA IDs.
*   **Adapter Module (`backend/pkg/integration/abdm`)**:
    *   Authenticates with ABDM Sandbox.
    *   Submits consent requests to the parent.
    *   Retrieves digital health records (FHIR DiagnosticReport/Observation bundles) from Health Information Providers (HIP).
    *   Ingests them directly into the Fasten backend database.

### 5.2 Portugal: SNS24 Gateway
*   **Standard**: Portuguese Ministry of Health API (Serviços Partilhados do Ministério da Saúde).
*   **Auth**: Chave Móvel Digital (CMD) authentication.
*   **Adapter Module (`backend/pkg/integration/sns24`)**:
    *   Uses custom OAuth endpoints configured for CMD.
    *   Downloads medication receipts and vaccination records in standard EU FHIR structures.

---

## 6. Development & Build Accelerators

To minimize manual coding overhead and speed up the production build, we utilize the following development accelerators:

### 6.1 Google Antigravity (AGY) SDK Multi-Agent Orchestration
*   **Purpose**: Deploy autonomous AI subagents to concurrent workflows.
*   **Usage**: Activate the `google-antigravity-sdk` skill to provision specialized subagents:
    *   `UI-UX Builder Agent`: Responsible for generating NativeWind-styled React Native components.
    *   `ABDM Gateway Agent`: Dedicated to testing and mocking ABDM/SNS24 HTTP request-response flows in the sandbox.
    *   `Compliance Auditor Agent`: Continuously reviews GORM schemas against HIPAA Field-Level Encryption constraints and monitors the write-only Audit log formats.

### 6.2 Tygo TypeScript Generator
*   **Purpose**: Accelerate frontend-backend integration and guarantee full type-safety.
*   **Usage**: Defined in `tygo.yaml`. Runs automatically via `make generate-backend`. Converts Go domain models (`backend/domain/model/*.go`) directly into TypeScript types stored in `frontend/src/app/models/types.ts`. This eliminates manual API contract writing.

### 6.3 Native Mobile UI/UX Boilerplates
*   **Framework**: Expo Prebuild templates integrated with **Tamagui** or **NativeWind** component sheets.
*   **Benefits**: Out-of-the-box support for dark modes, sleek gradients, gesture animations, and local encrypted SQLite setups.

### 6.4 Devcontainer & Nix Packages
*   **Docker Containerization**: Fasten's `.devcontainer` configuration spins up Go, Node, Yarn, and a local PostgreSQL cluster in under 3 minutes, resolving environmental setup bugs.
*   **Nix Flake**: Run `nix develop` to provision isolated path environments for native testing.
