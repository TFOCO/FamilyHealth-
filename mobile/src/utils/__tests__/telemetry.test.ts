import { 
  getFallbackName, 
  getMemberDetails, 
  computeAverages, 
  processTelemetryForChart,
  ChartCollection
} from '../telemetry';
import { VitalTelemetry, FamilyLink } from '../../../types';

describe('Telemetry Utilities (telemetry.ts)', () => {
  describe('getFallbackName', () => {
    test('resolves subject IDs or relation string to correct fallback name', () => {
      expect(getFallbackName(201, 'Father')).toBe('Aarav Sharma');
      expect(getFallbackName(202, 'Mother')).toBe('Priya Sharma');
      expect(getFallbackName(203, 'Son')).toBe('Rohan Sharma');
      expect(getFallbackName(999, 'Grandfather')).toBe('Grandfather Member');
      expect(getFallbackName(999, '')).toBe('Family Member');
    });
  });

  describe('getMemberDetails', () => {
    test('augments a link with name, avatar and status', () => {
      const link: FamilyLink = {
        id: 1,
        created_at: '2026-01-10T10:00:00Z',
        sponsor_id: 100,
        subject_id: 201,
        relation: 'Father',
        access_role: 'admin'
      };

      const details = getMemberDetails(link);
      expect(details.name).toBe('Aarav Sharma');
      expect(details.avatar).toBe('AS');
      expect(details.status).toBe('Normal');
    });

    test('preserves existing name, avatar and status if supplied', () => {
      const link: FamilyLink & { name?: string; status?: string; avatar?: string } = {
        id: 1,
        created_at: '2026-01-10T10:00:00Z',
        sponsor_id: 100,
        subject_id: 201,
        relation: 'Father',
        access_role: 'admin',
        name: 'John Doe',
        avatar: 'JD',
        status: 'Requires Review'
      };

      const details = getMemberDetails(link);
      expect(details.name).toBe('John Doe');
      expect(details.avatar).toBe('JD');
      expect(details.status).toBe('Requires Review');
    });
  });

  describe('computeAverages', () => {
    test('returns default averages when vitals list is empty', () => {
      const averages = computeAverages([]);
      expect(averages.BP.systolic).toBe(121);
      expect(averages.BP.diastolic).toBe(80);
      expect(averages.Glucose).toBe(110);
      expect(averages.HR).toBe(72);
    });

    test('correctly computes average metrics from single-metric vitals', () => {
      const vitals: VitalTelemetry[] = [
        { id: 1, subject_id: 201, vital_type: 'blood_pressure', value_metric: 130, value_unit: 'mmHg', context_data: '{"diastolic":85}', recorded_at: '2026-06-16T08:00:00Z' },
        { id: 2, subject_id: 201, vital_type: 'blood_pressure', value_metric: 120, value_unit: 'mmHg', context_data: '{"diastolic":75}', recorded_at: '2026-06-16T09:00:00Z' },
        { id: 3, subject_id: 201, vital_type: 'blood_glucose', value_metric: 120, value_unit: 'mg/dL', context_data: '{}', recorded_at: '2026-06-16T08:00:00Z' },
        { id: 4, subject_id: 201, vital_type: 'blood_glucose', value_metric: 100, value_unit: 'mg/dL', context_data: '{}', recorded_at: '2026-06-16T09:00:00Z' },
        { id: 5, subject_id: 201, vital_type: 'heart_rate', value_metric: 80, value_unit: 'bpm', context_data: '{}', recorded_at: '2026-06-16T08:00:00Z' },
        { id: 6, subject_id: 201, vital_type: 'heart_rate', value_metric: 70, value_unit: 'bpm', context_data: '{}', recorded_at: '2026-06-16T09:00:00Z' }
      ];

      const averages = computeAverages(vitals);
      expect(averages.BP.systolic).toBe(125); // (130+120)/2
      expect(averages.BP.diastolic).toBe(80);  // (85+75)/2
      expect(averages.Glucose).toBe(110);      // (120+100)/2
      expect(averages.HR).toBe(75);            // (80+70)/2
    });

    test('correctly parses multi_biometrics and averages it into metrics', () => {
      const vitals: VitalTelemetry[] = [
        { id: 1, subject_id: 201, vital_type: 'multi_biometrics', value_metric: 140, value_unit: 'mixed', context_data: '{"diastolic":90,"glucose":150}', recorded_at: '2026-06-16T08:00:00Z' },
        { id: 2, subject_id: 201, vital_type: 'multi_biometrics', value_metric: 120, value_unit: 'mixed', context_data: '{"diastolic":80,"glucose":130}', recorded_at: '2026-06-16T09:00:00Z' }
      ];

      const averages = computeAverages(vitals);
      expect(averages.BP.systolic).toBe(130);
      expect(averages.BP.diastolic).toBe(85);
      expect(averages.Glucose).toBe(140);
      expect(averages.HR).toBe(72); // unchanged default
    });
  });

  describe('processTelemetryForChart', () => {
    const fallback: ChartCollection = {
      BP: [{ day: 'Mon', systolic: 115, diastolic: 75 }, { day: 'Tue', systolic: 120, diastolic: 80 }],
      Glucose: [{ day: 'Mon', val: 90 }, { day: 'Tue', val: 100 }],
      HR: [{ day: 'Mon', val: 70 }, { day: 'Tue', val: 75 }]
    } as any;

    test('returns fallbackData if records list is empty', () => {
      const result = processTelemetryForChart([], fallback);
      expect(result).toEqual(fallback);
    });

    test('processes record arrays and groups by day of week correctly', () => {
      // 2026-06-15 is a Monday (getDay() === 1)
      // 2026-06-16 is a Tuesday (getDay() === 2)
      const vitals: VitalTelemetry[] = [
        { id: 1, subject_id: 201, vital_type: 'blood_pressure', value_metric: 130, value_unit: 'mmHg', context_data: '{"diastolic":90}', recorded_at: '2026-06-15T10:00:00Z' },
        { id: 2, subject_id: 201, vital_type: 'blood_pressure', value_metric: 120, value_unit: 'mmHg', context_data: '{"diastolic":80}', recorded_at: '2026-06-15T15:00:00Z' },
        { id: 3, subject_id: 201, vital_type: 'blood_glucose', value_metric: 140, value_unit: 'mg/dL', context_data: '{}', recorded_at: '2026-06-16T10:00:00Z' }
      ];

      const chart = processTelemetryForChart(vitals, fallback);
      // Monday average BP: sys (130+120)/2 = 125, dia (90+80)/2 = 85
      expect(chart.BP[1]).toEqual({ day: 'Mon', systolic: 125, diastolic: 85 });
      // Tuesday glucose: 140
      expect(chart.Glucose[2]).toEqual({ day: 'Tue', val: 140 });
      // Sunday (index 0) should have default values
      expect(chart.BP[0]).toEqual({ day: 'Sun', systolic: 120, diastolic: 80 });
      expect(chart.Glucose[0]).toEqual({ day: 'Sun', val: 100 });
      expect(chart.HR[0]).toEqual({ day: 'Sun', val: 72 });
    });
  });
});
