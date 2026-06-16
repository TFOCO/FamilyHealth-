import { extractPrescriptionLayout, parseRawTextToStructured } from '../edgeOcr';

describe('Offline Edge OCR Mock (edgeOcr)', () => {
  
  test('parseRawTextToStructured extracts patient, doctor, and dosage correctly', () => {
    const rawText = [
      "METROPOLIS HEALTH",
      "Dr. Clark Kent, MD",
      "Patient: Bruce Wayne",
      "Date: 2026-06-12",
      "Lisinopril 10mg",
      "Take 1 tablet daily by mouth.",
      "Qty: 30 | Refills: 5"
    ].join('\n');

    const structured = parseRawTextToStructured(rawText);

    expect(structured.patientName).toBe('Bruce Wayne');
    expect(structured.doctorName).toBe('Dr. Clark Kent, MD');
    expect(structured.medicationName).toBe('Lisinopril');
    expect(structured.dosage).toBe('10mg');
    expect(structured.frequency).toBe('once daily');
    expect(structured.refills).toBe(5);
    expect(structured.date).toBe('2026-06-12');
    expect(structured.instructions).toBe('Take 1 tablet daily by mouth.');
  });

  test('extractPrescriptionLayout parses metformin preset correctly', async () => {
    const imageUri = 'file:///assets/mock/prescription_metformin.png';
    const result = await extractPrescriptionLayout(imageUri);

    expect(result.rawText).toContain('Metformin 500mg');
    expect(result.confidence).toBeGreaterThan(0.9);
    expect(result.structured.medicationName).toBe('Metformin');
    expect(result.structured.dosage).toBe('500mg');
    expect(result.structured.frequency).toBe('twice daily');
    expect(result.structured.patientName).toBe('John Smith');
    expect(result.structured.refills).toBe(3);
    
    // Check that layout coordinates are returned
    expect(result.blocks.length).toBeGreaterThan(0);
    expect(result.blocks[0].boundingBox).toBeDefined();
    expect(result.blocks[0].lines[0].text).toBe('CITY HEALTH CLINIC');
  });

  test('extractPrescriptionLayout parses amlodipine preset correctly', async () => {
    const imageUri = 'assets/mock/prescription_amlodipine.jpg';
    const result = await extractPrescriptionLayout(imageUri);

    expect(result.structured.medicationName).toBe('Amlodipine');
    expect(result.structured.dosage).toBe('5mg');
    expect(result.structured.frequency).toBe('once daily');
    expect(result.structured.patientName).toBe('Jane Smith');
    expect(result.structured.refills).toBe(5);
  });

  test('extractPrescriptionLayout parses penicillin preset correctly', async () => {
    const imageUri = 'prescription_penicillin';
    const result = await extractPrescriptionLayout(imageUri);

    expect(result.structured.medicationName).toBe('Penicillin V Potassium');
    expect(result.structured.dosage).toBe('250mg');
    expect(result.structured.frequency).toBe('every 6 hours');
    expect(result.structured.patientName).toBe('Bruce Banner');
    expect(result.structured.refills).toBe(0);
  });

  test('extractPrescriptionLayout parses custom mock dynamic text URL correctly', async () => {
    const customText = [
      "CLINIC: Gotham General",
      "Dr. Harvey Dent",
      "Patient: Selina Kyle",
      "Ibuprofen 400mg",
      "Take 1 tablet every 8 hours for pain",
      "Refills: 2"
    ].join('\n');

    const encodedText = encodeURIComponent(customText);
    const imageUri = `mock://text?payload=${encodedText}`;

    const result = await extractPrescriptionLayout(imageUri);

    expect(result.structured.patientName).toBe('Selina Kyle');
    expect(result.structured.doctorName).toBe('Dr. Harvey Dent');
    expect(result.structured.medicationName).toBe('Ibuprofen');
    expect(result.structured.dosage).toBe('400mg');
    expect(result.structured.frequency).toBe('every 8 hours');
    expect(result.structured.refills).toBe(2);
    expect(result.structured.instructions).toBe('Take 1 tablet every 8 hours for pain');
  });

  test('extractPrescriptionLayout falls back to default layout if unknown image URI is passed', async () => {
    const imageUri = 'file:///random/unregistered/photo.jpg';
    const result = await extractPrescriptionLayout(imageUri);

    expect(result.structured.medicationName).toBe('Aspirin');
    expect(result.structured.dosage).toBe('81mg');
    expect(result.structured.frequency).toBe('once daily');
    expect(result.structured.patientName).toBe('Unknown Patient');
  });
});
