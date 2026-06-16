import { FamilyLink, VitalTelemetry } from '../../types';

export const getFallbackName = (subjectId: number, relation: string): string => {
  const relLower = (relation || '').toLowerCase();
  if (subjectId === 201 || relLower === 'father') return 'Aarav Sharma';
  if (subjectId === 202 || relLower === 'mother') return 'Priya Sharma';
  if (subjectId === 203 || relLower === 'son') return 'Rohan Sharma';
  return `${relation || 'Family'} Member`;
};

export const getMemberDetails = (link: FamilyLink & { name?: string; status?: string; avatar?: string }) => {
  const name = link.name || getFallbackName(link.subject_id, link.relation);
  const avatar = link.avatar || name.split(' ').map(n => n[0]).join('').substring(0, 2).toUpperCase();
  const status = link.status || 'Normal';
  return { name, avatar, status };
};

export interface ChartDayData {
  day: string;
  systolic?: number;
  diastolic?: number;
  val?: number;
}

export interface ChartCollection {
  BP: { day: string; systolic: number; diastolic: number }[];
  Glucose: { day: string; val: number }[];
  HR: { day: string; val: number }[];
}

export const processTelemetryForChart = (records: VitalTelemetry[], fallbackData: ChartCollection): ChartCollection => {
  if (!records || records.length === 0) {
    return fallbackData;
  }

  const days = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
  
  const bpChart = days.map(day => ({ day, systolic: 0, diastolic: 0, count: 0 }));
  const glucoseChart = days.map(day => ({ day, val: 0, count: 0 }));
  const hrChart = days.map(day => ({ day, val: 0, count: 0 }));

  records.forEach(v => {
    const date = new Date(v.recorded_at);
    const dayIndex = date.getDay(); // 0-6
    if (isNaN(dayIndex)) return; // skip invalid dates

    if (v.vital_type === 'blood_pressure') {
      let diastolicVal = 80;
      try {
        const parsed = JSON.parse(v.context_data);
        if (parsed && typeof parsed.diastolic === 'number') {
          diastolicVal = parsed.diastolic;
        }
      } catch (e) {}
      
      bpChart[dayIndex].systolic += v.value_metric;
      bpChart[dayIndex].diastolic += diastolicVal;
      bpChart[dayIndex].count++;
    } else if (v.vital_type === 'blood_glucose') {
      glucoseChart[dayIndex].val += v.value_metric;
      glucoseChart[dayIndex].count++;
    } else if (v.vital_type === 'heart_rate') {
      hrChart[dayIndex].val += v.value_metric;
      hrChart[dayIndex].count++;
    } else if (v.vital_type === 'multi_biometrics') {
      let diastolicVal = 80;
      let glucoseVal = 0;
      try {
        const parsed = JSON.parse(v.context_data);
        if (parsed) {
          if (typeof parsed.diastolic === 'number') {
            diastolicVal = parsed.diastolic;
          }
          if (typeof parsed.glucose === 'number') {
            glucoseVal = parsed.glucose;
          }
        }
      } catch (e) {}

      bpChart[dayIndex].systolic += v.value_metric;
      bpChart[dayIndex].diastolic += diastolicVal;
      bpChart[dayIndex].count++;

      if (glucoseVal > 0) {
        glucoseChart[dayIndex].val += glucoseVal;
        glucoseChart[dayIndex].count++;
      }
    }
  });

  const finalBP = bpChart.map(item => ({
    day: item.day,
    systolic: item.count > 0 ? Math.round(item.systolic / item.count) : 120,
    diastolic: item.count > 0 ? Math.round(item.diastolic / item.count) : 80,
  }));

  const finalGlucose = glucoseChart.map(item => ({
    day: item.day,
    val: item.count > 0 ? Math.round(item.val / item.count) : 100,
  }));

  const finalHR = hrChart.map(item => ({
    day: item.day,
    val: item.count > 0 ? Math.round(item.val / item.count) : 72,
  }));

  return {
    BP: finalBP,
    Glucose: finalGlucose,
    HR: finalHR,
  };
};

export const computeAverages = (vitals: VitalTelemetry[]) => {
  const averages = {
    BP: { systolic: 121, diastolic: 80 },
    Glucose: 110,
    HR: 72,
  };
  
  if (!vitals || vitals.length === 0) {
    return averages;
  }

  let bpCount = 0, bpSysSum = 0, bpDiaSum = 0;
  let glucoseCount = 0, glucoseSum = 0;
  let hrCount = 0, hrSum = 0;

  vitals.forEach(v => {
    if (v.vital_type === 'blood_pressure') {
      let diastolicVal = 80;
      try {
        const parsed = JSON.parse(v.context_data);
        if (parsed && typeof parsed.diastolic === 'number') diastolicVal = parsed.diastolic;
      } catch (e) {}
      bpSysSum += v.value_metric;
      bpDiaSum += diastolicVal;
      bpCount++;
    } else if (v.vital_type === 'blood_glucose') {
      glucoseSum += v.value_metric;
      glucoseCount++;
    } else if (v.vital_type === 'heart_rate') {
      hrSum += v.value_metric;
      hrCount++;
    } else if (v.vital_type === 'multi_biometrics') {
      let diastolicVal = 80;
      let glucoseVal = 0;
      try {
        const parsed = JSON.parse(v.context_data);
        if (parsed) {
          if (typeof parsed.diastolic === 'number') diastolicVal = parsed.diastolic;
          if (typeof parsed.glucose === 'number') glucoseVal = parsed.glucose;
        }
      } catch (e) {}
      bpSysSum += v.value_metric;
      bpDiaSum += diastolicVal;
      bpCount++;
      if (glucoseVal > 0) {
        glucoseSum += glucoseVal;
        glucoseCount++;
      }
    }
  });

  if (bpCount > 0) {
    averages.BP.systolic = Math.round(bpSysSum / bpCount);
    averages.BP.diastolic = Math.round(bpDiaSum / bpCount);
  }
  if (glucoseCount > 0) {
    averages.Glucose = Math.round(glucoseSum / glucoseCount);
  }
  if (hrCount > 0) {
    averages.HR = Math.round(hrSum / hrCount);
  }

  return averages;
};
