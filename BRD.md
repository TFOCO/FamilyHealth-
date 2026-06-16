# Business Requirements Document (BRD)
## Project: FamilyHealth+ (Diaspora-First Family Care Platform)
**Version:** 1.0  
**Date:** June 16, 2026  
**Author:** AI Systems Architect  

---

## 1. Executive Summary & Vision Statement

### 1.1 Vision Statement
To bridge the distance for the global Indian and Portuguese diaspora by providing a unified, AI-driven family health coordination platform. FamilyHealth+ enables sponsors abroad to actively monitor, coordinate, and finance the healthcare of their aging parents and family members back home, ensuring transparency, early intervention, and peace of mind.

### 1.2 The Problem
1.  **Caregiver Distance Gap**: Millions of high-earning immigrants (diaspora) send remittances home for healthcare but have zero visibility into their parents' daily health, medical records, or prescription adherence.
2.  **Fragmented Health History**: Parents visit multiple clinics/hospitals in India (or Portugal). Records are physical, handwritten, or scattered across disparate portal logins.
3.  **Complex Medical Jargon**: Non-medical sponsors receive confusing lab reports or prescription photos over WhatsApp and cannot easily interpret the severity or required follow-ups.
4.  **Fund Diversion**: Money sent home for healthcare is often diverted to other household expenses, leading to delayed medical checks or skipped treatments.

### 1.3 The USP & MOAT (The "Unimaginable" Capabilities)
*   **Active Cross-Border Sponsor Portal**: Dedicated views for the overseas sponsor (funding/USD) and the parent/local caregiver (INR/health data), with timezone-aware reminders.
*   **The AI Clinical Translator & Watchdog**: AI that translates complex, mixed-language, or handwritten medical charts into a clean "Diaspora Summary" highlighting *what to ask your parent on today's call*.
*   **Cross-Border Drug Equivalence Engine**: Instantly translates local Indian brand-name drugs to international equivalents to help sponsors cross-reference safety and usage.
*   **Health Escrow & Vetted Services**: Direct prepayment for prescriptions, lab tests, and home-care visits in India, ensuring funds are strictly used for care.
*   **Emergency Auto-Pilot**: A physical QR keychain/card for the parent. Scanned by any responder, it notifies the sponsor immediately (via WhatsApp/SMS/Call) and displays critical triage data.

---

## 2. User Personas

| Persona | Role | Core Needs | Technical Literacy |
| :--- | :--- | :--- | :--- |
| **Sponsor (e.g., Rajesh in US)** | Pays bills, monitors trends remotely | Complete health status clarity, direct payment control, proactive warnings. | High (Mobile/Web) |
| **Parent (e.g., Mrs. Sharma in Delhi)** | Measures vitals, visits local clinics | Simple interfaces, automated reminders, emergency support. | Low-Medium (Mobile/Tablet) |
| **Local Caregiver / Nurse** | Enters data, conducts home checks | Task checklists, easy photo uploading, record synchronization. | Medium (Mobile) |
| **Local Doctor / Clinic** | Consults, prescribes | Clean medical timeline, access to historical vitals during visits. | High (Desktop/FHIR Portal) |

---

## 3. Detailed Functional Requirements

### 3.1 Diaspora-Sponsor Portal & Care Coordination
*   **FR-1.1: Multi-Profile Family Aggregation**: Sponsor can link multiple family members (e.g., Mother, Father, Child) under a single dashboard.
*   **FR-1.2: Dual-Role Access (Sponsor vs. Subject)**: Sponsors can view data, update settings, and fund care, but only the Subject (or authorized local doctor) can log medical data (to preserve integrity).
*   **FR-1.3: Dual Currency & Timezone Sync**: Dashboard displays local time/vitals logs and matches them with the Sponsor's current timezone. Payments are accepted in USD/EUR and disbursed in INR.

### 3.2 AI-Powered Health Capabilities
*   **FR-2.1: Clinical Translation Engine**:
    *   Sponsor uploads a PDF/image of a prescription or lab report.
    *   AI extracts key entities (diagnoses, medications, dosages).
    *   AI outputs a patient-friendly summary: *"Your father was diagnosed with Hypertension. He needs to take Tab. Amlodipine 5mg once daily after breakfast. Do not miss this dose."*
*   **FR-2.2: AI Passive Trend Watchdog**:
    *   Monitors vital sign telemetry (blood pressure, blood glucose).
    *   If readings trend out of historical normal bounds (e.g., diastolic rising over 3 days), AI alerts the Sponsor: *"System Alert: Diastolic BP is trending upward. We suggest asking your father if he has had headaches or skipped his medication."*
*   **FR-2.3: Cross-Border Drug Equivalence Engine**:
    *   Maps Indian generic/brand names (e.g., *Glycomet*) to international names (e.g., *Metformin* / *Glucophage*).
    *   Provides drug-interaction checks against other active medications.

### 3.3 Financial & Health Escrow (Care Funding)
*   **FR-3.1: Vetted Local Booking**: Sponsor can book home health checkups, blood tests, or medicine deliveries directly via partner APIs in India/Portugal.
*   **FR-3.2: Direct Escrow Disbursements**: Money is paid directly to the service provider (e.g., diagnostic lab or pharmacy) instead of cash transfers to the parent.

### 3.4 Emergency Auto-Pilot & QR Access
*   **FR-4.1: Emergency QR Generation**: The app generates a secure, unique QR code for each parent.
*   **FR-4.2: Triage Access & Real-Time Alerting**:
    *   When scanned, it displays critical health data (Blood Type, Allergies, Current Meds, Emergency Contacts).
    *   Simultaneously triggers a webhook that sends an automated WhatsApp message and voice call to the Sponsor abroad with the parent's GPS location.

---

## 4. Non-Functional & Regulatory Requirements

### 4.1 Data Security & Privacy
*   **NFR-1.1: Local Encryption (SQLCipher)**: All health data stored on user devices must be encrypted using 256-bit AES keys.
*   **NFR-1.2: End-to-End Transit Security**: TLS 1.3 enforced for all API calls.

### 4.2 Regulatory Compliance
*   **NFR-2.1: GDPR (EU/Portugal)**: Ensure right to be forgotten, user consent logs, and localized database hosting options for EU users.
*   **NFR-2.2: DPDP Act (India)**: Comply with India's Digital Personal Data Protection Act requirements for clear consent, language localization, and data principal rights.

### 4.3 Reliability & Offline Capability
*   **NFR-3.1: Offline-First Synchronization**: App must function offline for parents (whose network connections may be spotty). Vitals logs must sync automatically once a connection is re-established.
