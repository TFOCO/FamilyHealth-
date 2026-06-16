import React, { useState } from 'react';
import { 
  View, 
  Text, 
  ScrollView, 
  TextInput, 
  TouchableOpacity, 
  Alert, 
  ActivityIndicator 
} from 'react-native';
import { useTranslation } from 'react-i18next';
import { 
  Activity, 
  CheckSquare, 
  Square, 
  QrCode, 
  Heart, 
  Smartphone, 
  AlertTriangle, 
  Thermometer, 
  FileText 
} from 'lucide-react-native';
import { VitalTelemetry, EmergencyQR } from '../../types';
import { fetchWithAuth, API_BASE_URL } from '../utils/secureStore';

// Mock Emergency QR profile mapping to the Go struct
const mockEmergencyQR: EmergencyQR = {
  id: 1,
  subject_id: 201,
  qr_hash: '9f8e7d6c5b4a3f2e1d0c',
  blood_group: 'O+',
  allergies: 'Penicillin, Peanuts, Sulfa Drugs',
  active_meds: 'Metformin 500mg, Lisinopril 10mg, Atorvastatin 20mg',
  sponsor_phone: '+1 (555) 019-2834',
  is_active: true,
};

export default function ParentDashboard() {
  const { t, i18n } = useTranslation();
  const [selectedLanguage, setSelectedLanguage] = useState(i18n.language);

  // Vitals Form State
  const [systolic, setSystolic] = useState('');
  const [diastolic, setDiastolic] = useState('');
  const [glucose, setGlucose] = useState('');
  const [temperature, setTemperature] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitSuccess, setSubmitSuccess] = useState(false);

  // Medication Checklist State
  const [meds, setMeds] = useState([
    { id: 1, nameKey: 'parent.morningMeds', checked: false },
    { id: 2, nameKey: 'parent.afternoonMeds', checked: false },
    { id: 3, nameKey: 'parent.eveningMeds', checked: false },
  ]);

  const toggleMed = (id: number) => {
    setMeds(prev => prev.map(m => m.id === id ? { ...m, checked: !m.checked } : m));
  };

  const completedCount = meds.filter(m => m.checked).length;

  const handleVitalsSubmit = async () => {
    if (!systolic || !diastolic || !glucose || !temperature) {
      Alert.alert('Validation Error', 'Please fill in all vitals fields before submitting.');
      return;
    }

    setIsSubmitting(true);
    
    const telemetryRecord: Omit<VitalTelemetry, 'id'> = {
      subject_id: mockEmergencyQR.subject_id,
      vital_type: 'multi_biometrics',
      value_metric: parseFloat(systolic),
      value_unit: 'mixed',
      context_data: JSON.stringify({
        diastolic: parseFloat(diastolic),
        glucose: parseFloat(glucose),
        temperature: parseFloat(temperature),
      }),
      recorded_at: new Date().toISOString(),
    };

    try {
      console.log('Sending vitals telemetry to Go Backend:', telemetryRecord);
      const response = await fetchWithAuth(`${API_BASE_URL}/api/v1/health/telemetry`, {
        method: 'POST',
        body: JSON.stringify(telemetryRecord),
      });

      if (!response.ok) {
        throw new Error(`Server responded with status ${response.status}`);
      }

      const json = await response.json();
      if (json && json.success === false) {
        throw new Error(json.error || 'Unknown server error');
      }

      setSubmitSuccess(true);
      setSystolic('');
      setDiastolic('');
      setGlucose('');
      setTemperature('');

      setTimeout(() => setSubmitSuccess(false), 5000);
    } catch (error: any) {
      console.error('Error submitting vitals telemetry:', error);
      Alert.alert('Submission Failed', error.message || 'An error occurred while logging vitals.');
    } finally {
      setIsSubmitting(false);
    }
  };

  const changeLanguage = (lng: string) => {
    i18n.changeLanguage(lng);
    setSelectedLanguage(lng);
  };

  return (
    <ScrollView className="flex-1 bg-slate-50 pb-10">
      {/* Header */}
      <View className="bg-emerald-700 px-6 pt-12 pb-6 rounded-b-[24px] shadow-lg">
        <View className="flex-row justify-between items-center">
          <View>
            <Text className="text-white text-2xl font-bold tracking-tight">{t('parent.title')}</Text>
            <Text className="text-emerald-100 text-sm mt-0.5">{t('parent.subTitle')}</Text>
          </View>
          <View className="bg-emerald-600 px-3 py-1.5 rounded-full border border-emerald-500">
            <Text className="text-emerald-50 text-xs font-semibold uppercase">{t('common.parent')}</Text>
          </View>
        </View>

        {/* Language Selection */}
        <View className="flex-row items-center justify-end mt-4 space-x-2">
          {['en', 'hi', 'pt'].map((lang) => (
            <TouchableOpacity
              key={lang}
              onPress={() => changeLanguage(lang)}
              className={`px-3 py-1 rounded-md ${
                selectedLanguage === lang ? 'bg-white' : 'bg-emerald-800'
              }`}
            >
              <Text
                className={`text-xs font-bold ${
                  selectedLanguage === lang ? 'text-emerald-800' : 'text-emerald-100'
                }`}
              >
                {lang.toUpperCase()}
              </Text>
            </TouchableOpacity>
          ))}
        </View>
      </View>

      {/* Main Container */}
      <View className="px-4 py-6 space-y-6">
        
        {/* Vitals Form Card */}
        <View className="bg-white p-5 rounded-2xl shadow-sm border border-slate-100">
          <View className="flex-row items-center mb-4">
            <Activity size={22} className="text-emerald-700 mr-2" />
            <Text className="text-slate-800 text-lg font-bold">{t('parent.vitalsForm')}</Text>
          </View>

          {submitSuccess && (
            <View className="mb-4 bg-emerald-50 border border-emerald-200 p-3 rounded-xl">
              <Text className="text-emerald-800 text-sm text-center font-semibold">
                {t('parent.vitalsSuccess')}
              </Text>
            </View>
          )}

          <View className="space-y-4">
            {/* BP Input row */}
            <View className="flex-row space-x-3">
              <View className="flex-1">
                <Text className="text-slate-500 text-xs font-semibold mb-1">{t('parent.systolic')}</Text>
                <TextInput
                  value={systolic}
                  onChangeText={setSystolic}
                  keyboardType="numeric"
                  placeholder="e.g. 120"
                  className="bg-slate-50 border border-slate-200 rounded-xl px-4 py-3 text-slate-800 focus:border-emerald-500"
                />
              </View>
              <View className="flex-1">
                <Text className="text-slate-500 text-xs font-semibold mb-1">{t('parent.diastolic')}</Text>
                <TextInput
                  value={diastolic}
                  onChangeText={setDiastolic}
                  keyboardType="numeric"
                  placeholder="e.g. 80"
                  className="bg-slate-50 border border-slate-200 rounded-xl px-4 py-3 text-slate-800 focus:border-emerald-500"
                />
              </View>
            </View>

            {/* Glucose input */}
            <View>
              <Text className="text-slate-500 text-xs font-semibold mb-1">{t('parent.glucoseVal')}</Text>
              <TextInput
                value={glucose}
                onChangeText={setGlucose}
                keyboardType="numeric"
                placeholder="e.g. 98"
                className="bg-slate-50 border border-slate-200 rounded-xl px-4 py-3 text-slate-800 focus:border-emerald-500"
              />
            </View>

            {/* Temperature input */}
            <View>
              <Text className="text-slate-500 text-xs font-semibold mb-1">{t('parent.tempVal')}</Text>
              <TextInput
                value={temperature}
                onChangeText={setTemperature}
                keyboardType="numeric"
                placeholder="e.g. 36.6"
                className="bg-slate-50 border border-slate-200 rounded-xl px-4 py-3 text-slate-800 focus:border-emerald-500"
              />
            </View>

            <TouchableOpacity
              onPress={handleVitalsSubmit}
              disabled={isSubmitting}
              className="w-full bg-emerald-700 py-3.5 rounded-xl items-center shadow-md active:bg-emerald-800 mt-2"
            >
              {isSubmitting ? (
                <ActivityIndicator color="white" />
              ) : (
                <Text className="text-white font-bold text-base">{t('parent.submitVitals')}</Text>
              )}
            </TouchableOpacity>
          </View>
        </View>

        {/* Medication Checklist Card */}
        <View className="bg-white p-5 rounded-2xl shadow-sm border border-slate-100">
          <View className="flex-row justify-between items-center mb-4">
            <View className="flex-row items-center">
              <CheckSquare size={22} className="text-emerald-700 mr-2" />
              <Text className="text-slate-800 text-lg font-bold">{t('parent.medsChecklist')}</Text>
            </View>
            <Text className="text-emerald-700 text-xs font-bold">
              {t('parent.medsProgress', { count: completedCount })}
            </Text>
          </View>

          {/* Progress Bar */}
          <View className="w-full bg-slate-100 h-2.5 rounded-full mb-5 overflow-hidden">
            <View 
              style={{ width: `${(completedCount / meds.length) * 100}%` }}
              className="bg-emerald-600 h-full rounded-full"
            />
          </View>

          {/* Checklist Items */}
          <View className="space-y-3">
            {meds.map((item) => (
              <TouchableOpacity
                key={item.id}
                onPress={() => toggleMed(item.id)}
                className={`flex-row items-center p-3 rounded-xl border ${
                  item.checked 
                    ? 'bg-emerald-50/50 border-emerald-200' 
                    : 'bg-slate-50 border-slate-100'
                }`}
              >
                {item.checked ? (
                  <CheckSquare size={20} className="text-emerald-700 mr-3" />
                ) : (
                  <Square size={20} className="text-slate-400 mr-3" />
                )}
                <Text className={`text-sm ${
                  item.checked ? 'text-slate-500 line-through' : 'text-slate-800 font-medium'
                }`}>
                  {t(item.nameKey)}
                </Text>
              </TouchableOpacity>
            ))}
          </View>
        </View>

        {/* Emergency Medical QR Profile */}
        <View className="bg-white p-5 rounded-2xl shadow-sm border border-slate-100">
          <View className="flex-row items-center mb-4">
            <QrCode size={22} className="text-rose-700 mr-2" />
            <Text className="text-slate-800 text-lg font-bold">{t('parent.emergencyQR')}</Text>
          </View>

          {/* Emergency Card Display */}
          <View className="bg-rose-50/50 border border-rose-100 p-5 rounded-2xl shadow-xs">
            
            {/* Header info */}
            <View className="flex-row justify-between items-center mb-4 pb-3 border-b border-rose-100">
              <View>
                <Text className="text-slate-800 font-bold text-base">EMERGENCY CARD</Text>
                <Text className="text-rose-700 text-xs font-semibold">{t('parent.qrStatus')}</Text>
              </View>
              <Heart size={24} className="text-rose-600 fill-rose-600" />
            </View>

            {/* Content list */}
            <View className="space-y-3.5">
              <View>
                <Text className="text-slate-400 text-[10px] font-bold uppercase tracking-wide">{t('parent.bloodGroup')}</Text>
                <Text className="text-slate-800 text-sm font-semibold">{mockEmergencyQR.blood_group}</Text>
              </View>

              <View>
                <Text className="text-slate-400 text-[10px] font-bold uppercase tracking-wide">{t('parent.allergies')}</Text>
                <Text className="text-slate-800 text-sm font-medium">{mockEmergencyQR.allergies}</Text>
              </View>

              <View>
                <Text className="text-slate-400 text-[10px] font-bold uppercase tracking-wide">{t('parent.activeMeds')}</Text>
                <Text className="text-slate-800 text-sm font-medium">{mockEmergencyQR.active_meds}</Text>
              </View>

              <View className="flex-row items-center space-x-2 pt-1">
                <Smartphone size={16} className="text-slate-400" />
                <View>
                  <Text className="text-slate-400 text-[10px] font-bold uppercase tracking-wide">{t('parent.sponsorContact')}</Text>
                  <Text className="text-slate-800 text-sm font-semibold">{mockEmergencyQR.sponsor_phone}</Text>
                </View>
              </View>
            </View>

            {/* QR Mockup visualization */}
            <View className="items-center justify-center mt-5 bg-white p-4 rounded-xl border border-rose-100/60 shadow-xs self-center">
              {/* Outer box */}
              <View className="w-32 h-32 border border-slate-300 p-2 rounded-lg items-center justify-center">
                {/* Visual lines representation of QR code */}
                <View className="w-full h-full flex-wrap flex-row">
                  {[...Array(64)].map((_, i) => {
                    const isFilled = (i % 2 === 0 && i % 3 !== 0) || i % 5 === 0 || i < 7 || i % 8 === 0;
                    return (
                      <View 
                        key={i} 
                        className={`w-3.5 h-3.5 m-0.5 rounded-xs ${isFilled ? 'bg-slate-800' : 'bg-white'}`} 
                      />
                    );
                  })}
                </View>
              </View>
              <Text className="text-[9px] text-slate-400 mt-2 font-mono">{mockEmergencyQR.qr_hash}</Text>
            </View>

            <Text className="text-slate-500 text-xs text-center mt-4 italic">
              {t('parent.qrInstructions')}
            </Text>
          </View>
        </View>
      </View>
    </ScrollView>
  );
}
