export interface OcrBoundingBox {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface OcrTextLine {
  text: string;
  confidence: number;
  boundingBox?: OcrBoundingBox;
}

export interface OcrTextBlock {
  lines: OcrTextLine[];
  boundingBox?: OcrBoundingBox;
}

export interface StructuredPrescription {
  patientName?: string;
  doctorName?: string;
  medicationName: string;
  dosage: string;
  frequency: string;
  refills?: number;
  date?: string;
  instructions?: string;
  rawText?: string;
}

export interface OcrExtractionResult {
  rawText: string;
  blocks: OcrTextBlock[];
  structured: StructuredPrescription;
  confidence: number;
}

/**
 * Parses raw text extracted from a prescription label into a structured JSON layout.
 * Uses robust regex and text pattern matching to populate medication details.
 */
export function parseRawTextToStructured(text: string): StructuredPrescription {
  const lines = text.split('\n');
  const result: StructuredPrescription = {
    medicationName: 'Unknown',
    dosage: 'Unknown',
    frequency: 'Unknown',
    rawText: text
  };

  // Regular expression heuristics
  const patientRegex = /(?:patient|pat):\s*([a-zA-Z\s]+)/i;
  const doctorRegex = /(?:dr|doctor|md):\s*([a-zA-Z\s.,]+)/i;
  const refillRegex = /(?:refills|refill|qty):\s*(\d+)/i;
  const dateRegex = /(\d{4}-\d{2}-\d{2})/i;
  
  // Pattern to find medication name followed by a dosage (e.g. Metformin 500mg, Lisinopril 10 mg)
  const medDosageRegex = /([a-zA-Z\s]+?)\s+(\d+(?:\s*(?:mg|g|mcg|ml|tab|caps|capsules|tablets)))\b/i;

  for (const line of lines) {
    const cleanLine = line.trim();
    if (!cleanLine) continue;

    const pMatch = cleanLine.match(patientRegex);
    if (pMatch && !result.patientName) {
      result.patientName = pMatch[1].trim();
      continue;
    }

    const dMatch = cleanLine.match(doctorRegex);
    if (dMatch && !result.doctorName) {
      result.doctorName = dMatch[1].trim();
      continue;
    }

    const rMatch = cleanLine.match(refillRegex);
    if (rMatch && result.refills === undefined) {
      result.refills = parseInt(rMatch[1].trim(), 10);
      continue;
    }

    const dateMatch = cleanLine.match(dateRegex);
    if (dateMatch && !result.date) {
      result.date = dateMatch[1].trim();
      continue;
    }

    const medMatch = cleanLine.match(medDosageRegex);
    if (medMatch && result.medicationName === 'Unknown') {
      const candidateMed = medMatch[1].trim();
      // Avoid matching keywords like Patient, Doctor, Date or Clinic as medication
      const lowerCandidate = candidateMed.toLowerCase();
      if (!lowerCandidate.includes('patient') && 
          !lowerCandidate.includes('dr.') && 
          !lowerCandidate.includes('doctor') &&
          !lowerCandidate.includes('clinic') &&
          !lowerCandidate.includes('center')) {
        result.medicationName = candidateMed;
        result.dosage = medMatch[2].trim();
      }
    }
  }

  // Identify instructions and frequency
  const instructionKeywords = ['take', 'apply', 'inhale', 'use', 'inject', 'consume', 'instill'];
  for (const line of lines) {
    const lowerLine = line.toLowerCase().trim();
    if (instructionKeywords.some(keyword => lowerLine.startsWith(keyword))) {
      result.instructions = line.trim();

      // Extrapolate frequency
      if (lowerLine.includes('twice daily') || lowerLine.includes('2x daily') || lowerLine.includes('2 times daily')) {
        result.frequency = 'twice daily';
      } else if (lowerLine.includes('three times daily') || lowerLine.includes('3x daily') || lowerLine.includes('3 times daily')) {
        result.frequency = 'three times daily';
      } else if (lowerLine.includes('every 8 hours') || lowerLine.includes('q8h')) {
        result.frequency = 'every 8 hours';
      } else if (lowerLine.includes('every 6 hours') || lowerLine.includes('q6h')) {
        result.frequency = 'every 6 hours';
      } else if (lowerLine.includes('daily') || lowerLine.includes('once daily') || lowerLine.includes('once a day')) {
        result.frequency = 'once daily';
      } else if (lowerLine.includes('every 12 hours') || lowerLine.includes('q12h') || lowerLine.includes('twice a day')) {
        result.frequency = 'every 12 hours';
      }
    }
  }

  return result;
}

/**
 * Mock offline OCR layout extractor that parses mock prescription labels into structured JSON.
 * Returns bounding boxes and text blocks for realistic on-device UI rendering.
 */
export async function extractPrescriptionLayout(imageUri: string): Promise<OcrExtractionResult> {
  // Simulate offline device processing delay (100ms to 300ms) to emulate CPU time
  await new Promise(resolve => setTimeout(resolve, 150));

  let rawText = '';
  let structured: StructuredPrescription = {
    medicationName: 'Unknown',
    dosage: 'Unknown',
    frequency: 'Unknown'
  };

  const normalizedUri = imageUri.toLowerCase();

  if (normalizedUri.includes('prescription_metformin')) {
    rawText = [
      "CITY HEALTH CLINIC",
      "Dr. Jane Doe, MD",
      "Patient: John Smith",
      "Date: 2026-05-15",
      "Metformin 500mg",
      "Take 1 tablet by mouth twice daily with meals.",
      "Qty: 60 | Refills: 3"
    ].join('\n');
    structured = {
      patientName: "John Smith",
      doctorName: "Dr. Jane Doe, MD",
      medicationName: "Metformin",
      dosage: "500mg",
      frequency: "twice daily",
      refills: 3,
      date: "2026-05-15",
      instructions: "Take 1 tablet by mouth twice daily with meals.",
      rawText: rawText
    };
  } else if (normalizedUri.includes('prescription_amlodipine')) {
    rawText = [
      "METRO CARDIOLOGY",
      "Dr. Alan Stark, MD",
      "Patient: Jane Smith",
      "Date: 2026-06-01",
      "Amlodipine 5mg",
      "Take 1 tablet daily in the morning.",
      "Qty: 30 | Refills: 5"
    ].join('\n');
    structured = {
      patientName: "Jane Smith",
      doctorName: "Dr. Alan Stark, MD",
      medicationName: "Amlodipine",
      dosage: "5mg",
      frequency: "once daily",
      refills: 5,
      date: "2026-06-01",
      instructions: "Take 1 tablet daily in the morning.",
      rawText: rawText
    };
  } else if (normalizedUri.includes('prescription_penicillin')) {
    rawText = [
      "EMERGENCY CARE CENTER",
      "Dr. Robert Bruce",
      "Patient: Bruce Banner",
      "Date: 2026-06-10",
      "Penicillin V Potassium 250mg",
      "Take 1 tablet every 6 hours on an empty stomach.",
      "Qty: 40 | Refills: 0"
    ].join('\n');
    structured = {
      patientName: "Bruce Banner",
      doctorName: "Dr. Robert Bruce",
      medicationName: "Penicillin V Potassium",
      dosage: "250mg",
      frequency: "every 6 hours",
      refills: 0,
      date: "2026-06-10",
      instructions: "Take 1 tablet every 6 hours on an empty stomach.",
      rawText: rawText
    };
  } else if (imageUri.startsWith('mock://text?') || imageUri.startsWith('data:text/')) {
    // Dynamic parsing from query parameters or data URIs to support custom unit testing
    let textContent = '';
    if (imageUri.startsWith('mock://text?')) {
      const queryPart = imageUri.split('?')[1] || '';
      const params = queryPart.split('&');
      for (const param of params) {
        const [key, value] = param.split('=');
        if (key === 'payload') {
          textContent = decodeURIComponent(value || '');
          break;
        }
      }
    } else {
      textContent = decodeURIComponent(imageUri.substring(imageUri.indexOf(',') + 1));
    }
    rawText = textContent;
    structured = parseRawTextToStructured(textContent);
  } else {
    // Default fallback mock
    rawText = [
      "GENERAL MEDICAL OUTPATIENT",
      "Dr. Unknown Doctor",
      "Patient: Unknown Patient",
      "Aspirin 81mg",
      "Take 1 tablet daily",
      "Refills: 0"
    ].join('\n');
    
    structured = {
      patientName: "Unknown Patient",
      doctorName: "Dr. Unknown Doctor",
      medicationName: "Aspirin",
      dosage: "81mg",
      frequency: "once daily",
      refills: 0,
      instructions: "Take 1 tablet daily",
      rawText: rawText
    };
  }

  // Generate realistic text blocks and line coordinates to mimic layout scanning
  const lines = rawText.split('\n');
  const blocks: OcrTextBlock[] = [];
  let currentY = 15;

  for (let i = 0; i < lines.length; i++) {
    const text = lines[i].trim();
    if (!text) continue;

    const lineLength = text.length;
    const lineWidth = lineLength * 8; // Simulating pixel width
    const lineX = 15;

    const line: OcrTextLine = {
      text,
      confidence: 0.95 + Math.random() * 0.049,
      boundingBox: {
        x: lineX,
        y: currentY,
        width: lineWidth,
        height: 18
      }
    };

    blocks.push({
      lines: [line],
      boundingBox: {
        x: lineX,
        y: currentY,
        width: lineWidth,
        height: 18
      }
    });

    currentY += 24; // Line spacing height
  }

  return {
    rawText,
    blocks,
    structured,
    confidence: 0.98
  };
}
