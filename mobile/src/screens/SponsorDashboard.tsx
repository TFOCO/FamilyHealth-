import React, { useState, useEffect } from 'react';
import { View, Text, ScrollView, TouchableOpacity, Alert, Linking, ActivityIndicator } from 'react-native';
import { useTranslation } from 'react-i18next';
import { 
  Users, 
  CreditCard, 
  TrendingUp, 
  ShieldCheck, 
  Activity, 
  Heart, 
  AlertCircle 
} from 'lucide-react-native';
import { FamilyLink, VitalTelemetry } from '../../types';
import { fetchWithAuth, API_BASE_URL } from '../utils/secureStore';
import { getMemberDetails, processTelemetryForChart, computeAverages } from '../utils/telemetry';

export default function SponsorDashboard() {
  const { t, i18n } = useTranslation();
  const [selectedLanguage, setSelectedLanguage] = useState(i18n.language);
  const [activeTelemetryTab, setActiveTelemetryTab] = useState<'BP' | 'Glucose' | 'HR'>('BP');

  // Real database states
  const [familyLinks, setFamilyLinks] = useState<FamilyLink[]>([]);
  const [vitals, setVitals] = useState<VitalTelemetry[]>([]);
  const [aiSummary, setAiSummary] = useState<any>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function loadData() {
      try {
        setIsLoading(true);
        setError(null);

        const linksRes = await fetchWithAuth(`${API_BASE_URL}/api/v1/family/links`);
        if (!linksRes.ok) throw new Error(`Family links API returned status ${linksRes.status}`);
        const linksJson = await linksRes.ok ? await linksRes.json() : null;
        const linksData: FamilyLink[] = Array.isArray(linksJson) ? linksJson : (linksJson?.data || []);

        const telemetryRes = await fetchWithAuth(`${API_BASE_URL}/api/v1/health/telemetry`);
        if (!telemetryRes.ok) throw new Error(`Telemetry API returned status ${telemetryRes.status}`);
        const telemetryJson = await telemetryRes.ok ? await telemetryRes.json() : null;
        const telemetryData: VitalTelemetry[] = Array.isArray(telemetryJson) ? telemetryJson : (telemetryJson?.data || []);

        // Fetch AI Summary
        try {
          const summaryRes = await fetchWithAuth(`${API_BASE_URL}/api/v1/health/summary?subject_id=201`);
          if (summaryRes.ok) {
            const summaryData = await summaryRes.json();
            setAiSummary(summaryData);
          }
        } catch (e) {
          console.log('Failed to load AI summary', e);
        }

        setFamilyLinks(linksData);
        setVitals(telemetryData);
      } catch (err: any) {
        console.error('Error loading sponsor dashboard data:', err);
        setError(err.message || 'Failed to load dashboard data.');
      } finally {
        setIsLoading(false);
      }
    }
    loadData();
  }, []);

  const changeLanguage = (lng: string) => {
    i18n.changeLanguage(lng);
    setSelectedLanguage(lng);
  };

  const handlePayNow = async () => {
    try {
      const res = await fetchWithAuth(`${API_BASE_URL}/api/v1/payments/escrow/initiate`, {
        method: 'POST',
        body: JSON.stringify({ sponsor_id: 1, subject_id: 201, amount: 450.50, currency: 'USD', method: 'CARD' }),
      });
      const data = await res.json();
      if (data.client_secret) {
        Alert.alert('Payment Initiated', `Stripe Client Secret received: ${data.client_secret}`);
        // In real app, launch Stripe SDK here
      }
    } catch (err: any) {
      Alert.alert(t('sponsor.paymentsTitle'), 'An error occurred initiating card payment: ' + err.message);
    }
  };

  const handlePayUPI = async () => {
    try {
      const res = await fetchWithAuth(`${API_BASE_URL}/api/v1/payments/escrow/initiate`, {
        method: 'POST',
        body: JSON.stringify({ sponsor_id: 1, subject_id: 201, amount: 450.50, currency: 'INR', method: 'UPI' }),
      });
      const data = await res.json();
      if (data.upi_link) {
        const supported = await Linking.canOpenURL(data.upi_link);
        if (supported) {
          Linking.openURL(data.upi_link);
        } else {
          Alert.alert('Payment', 'UPI payment apps are not installed. Link generated: ' + data.upi_link);
        }
      }
    } catch (err: any) {
      Alert.alert(t('sponsor.paymentsTitle'), 'An error occurred initiating UPI payment: ' + err.message);
    }
  };

  // Mock telemetry data points for rendering the dynamic SVG trend chart if no database data is found
  const defaultChartData = {
    BP: [
      { day: 'Mon', systolic: 118, diastolic: 78 },
      { day: 'Tue', systolic: 122, diastolic: 81 },
      { day: 'Wed', systolic: 125, diastolic: 83 },
      { day: 'Thu', systolic: 119, diastolic: 79 },
      { day: 'Fri', systolic: 120, diastolic: 80 },
      { day: 'Sat', systolic: 121, diastolic: 80 },
      { day: 'Sun', systolic: 124, diastolic: 82 },
    ],
    Glucose: [
      { day: 'Mon', val: 95 },
      { day: 'Tue', val: 110 },
      { day: 'Wed', val: 145 },
      { day: 'Thu', val: 102 },
      { day: 'Fri', val: 98 },
      { day: 'Sat', val: 115 },
      { day: 'Sun', val: 105 },
    ],
    HR: [
      { day: 'Mon', val: 68 },
      { day: 'Tue', val: 72 },
      { day: 'Wed', val: 75 },
      { day: 'Thu', val: 70 },
      { day: 'Fri', val: 72 },
      { day: 'Sat', val: 74 },
      { day: 'Sun', val: 71 },
    ],
  };

  const chartData = processTelemetryForChart(vitals, defaultChartData);
  const averages = computeAverages(vitals);


  return (
    <ScrollView className="flex-1 bg-slate-50 pb-10">
      {/* Header */}
      <View className="bg-teal-700 px-6 pt-12 pb-6 rounded-b-[24px] shadow-lg">
        <View className="flex-row justify-between items-center">
          <View>
            <Text className="text-white text-2xl font-bold tracking-tight">{t('sponsor.title')}</Text>
            <Text className="text-teal-100 text-sm mt-0.5">{t('sponsor.subTitle')}</Text>
          </View>
          {/* Status badge */}
          <View className="bg-teal-600 px-3 py-1.5 rounded-full border border-teal-500">
            <Text className="text-teal-50 text-xs font-semibold uppercase">{t('common.sponsor')}</Text>
          </View>
        </View>

        {/* Dynamic Language Switcher */}
        <View className="flex-row items-center justify-end mt-4 space-x-2">
          {['en', 'hi', 'pt'].map((lang) => (
            <TouchableOpacity
              key={lang}
              onPress={() => changeLanguage(lang)}
              className={`px-3 py-1 rounded-md ${
                selectedLanguage === lang ? 'bg-white' : 'bg-teal-800'
              }`}
            >
              <Text
                className={`text-xs font-bold ${
                  selectedLanguage === lang ? 'text-teal-800' : 'text-teal-100'
                }`}
              >
                {lang.toUpperCase()}
              </Text>
            </TouchableOpacity>
          ))}
        </View>
      </View>

      {/* Main Content Area */}
      <View className="px-4 py-6 space-y-6">
        {isLoading ? (
          <View className="py-20 items-center justify-center bg-white rounded-2xl border border-slate-100 shadow-sm">
            <ActivityIndicator size="large" color="#0f766e" />
            <Text className="text-slate-500 text-sm mt-3 font-semibold">Retrieving Family Health Records...</Text>
          </View>
        ) : error ? (
          <View className="p-5 items-center justify-center bg-white rounded-2xl border border-red-100 shadow-sm">
            <AlertCircle size={32} className="text-rose-600 mb-2" />
            <Text className="text-slate-800 text-sm font-bold text-center">Failed to Synchronize Records</Text>
            <Text className="text-slate-500 text-xs text-center mt-1">{error}</Text>
          </View>
        ) : (
          <>
            {/* Section 1: Family Summary List */}
            <View className="bg-white p-5 rounded-2xl shadow-sm border border-slate-100">
              <View className="flex-row items-center mb-4">
                <Users size={22} className="text-teal-700 mr-2" />
                <Text className="text-slate-800 text-lg font-bold">{t('sponsor.familySummary')}</Text>
              </View>

              <View className="space-y-3">
                {familyLinks.map((member) => {
                  const { name, avatar, status } = getMemberDetails(member);
                  return (
                    <View key={member.id} className="flex-row items-center justify-between p-3 bg-slate-50 rounded-xl border border-slate-100">
                      <View className="flex-row items-center space-x-3">
                        <View className="w-10 h-10 bg-teal-100 rounded-full items-center justify-center">
                          <Text className="text-teal-800 font-bold text-sm">{avatar}</Text>
                        </View>
                        <View>
                          <Text className="text-slate-800 font-semibold text-base">{name}</Text>
                          <Text className="text-slate-500 text-xs">{member.relation} • {member.access_role}</Text>
                        </View>
                      </View>
                      
                      <View className="items-end">
                        <View className={`px-2 py-0.5 rounded-full ${
                          status === 'Normal' ? 'bg-emerald-50 border border-emerald-200' : 'bg-amber-50 border border-amber-200'
                        }`}>
                          <Text className={`text-[10px] font-bold ${
                            status === 'Normal' ? 'text-emerald-700' : 'text-amber-700'
                          }`}>
                            {status}
                          </Text>
                        </View>
                      </View>
                    </View>
                  );
                })}
                {familyLinks.length === 0 && (
                  <Text className="text-slate-400 text-center py-4">No family members linked yet.</Text>
                )}
              </View>
            </View>

            {/* AI Clinical Summary */}
            {aiSummary && (
              <View className="bg-teal-50 p-5 rounded-2xl shadow-sm border border-teal-100">
                <View className="flex-row items-center mb-3">
                  <Activity size={22} className="text-teal-800 mr-2" />
                  <Text className="text-teal-900 text-lg font-bold">AI Clinical Summary</Text>
                </View>
                <Text className="text-teal-800 text-sm mb-3 leading-5">{aiSummary.summary}</Text>
                {aiSummary.red_flags && aiSummary.red_flags.length > 0 && (
                  <View className="bg-rose-50 p-3 rounded-lg border border-rose-100 mt-2">
                    <Text className="text-rose-800 text-xs font-bold mb-1">Red Flags / Action Items:</Text>
                    {aiSummary.red_flags.map((flag: string, i: number) => (
                      <Text key={i} className="text-rose-700 text-xs">• {flag}</Text>
                    ))}
                  </View>
                )}
              </View>
            )}

            {/* Section 2: Telemetry Trends Chart */}
            <View className="bg-white p-5 rounded-2xl shadow-sm border border-slate-100">
              <View className="flex-row items-center justify-between mb-4">
                <View className="flex-row items-center">
                  <TrendingUp size={22} className="text-teal-700 mr-2" />
                  <Text className="text-slate-800 text-lg font-bold">{t('sponsor.telemetryTitle')}</Text>
                </View>
                <Activity size={18} className="text-slate-400 animate-pulse" />
              </View>

              {/* Metric Selector Tabs */}
              <View className="flex-row bg-slate-100 p-1 rounded-xl mb-4">
                <TouchableOpacity 
                  onPress={() => setActiveTelemetryTab('BP')} 
                  className={`flex-1 py-2 rounded-lg items-center ${activeTelemetryTab === 'BP' ? 'bg-white shadow-xs' : ''}`}
                >
                  <Text className={`text-xs font-semibold ${activeTelemetryTab === 'BP' ? 'text-teal-700' : 'text-slate-500'}`}>
                    {t('sponsor.bloodPressure')}
                  </Text>
                </TouchableOpacity>
                <TouchableOpacity 
                  onPress={() => setActiveTelemetryTab('Glucose')} 
                  className={`flex-1 py-2 rounded-lg items-center ${activeTelemetryTab === 'Glucose' ? 'bg-white shadow-xs' : ''}`}
                >
                  <Text className={`text-xs font-semibold ${activeTelemetryTab === 'Glucose' ? 'text-teal-700' : 'text-slate-500'}`}>
                    {t('sponsor.glucose')}
                  </Text>
                </TouchableOpacity>
                <TouchableOpacity 
                  onPress={() => setActiveTelemetryTab('HR')} 
                  className={`flex-1 py-2 rounded-lg items-center ${activeTelemetryTab === 'HR' ? 'bg-white shadow-xs' : ''}`}
                >
                  <Text className={`text-xs font-semibold ${activeTelemetryTab === 'HR' ? 'text-teal-700' : 'text-slate-500'}`}>
                    {t('sponsor.heartRate')}
                  </Text>
                </TouchableOpacity>
              </View>

              {/* Visual Trend Chart (Stylized Bar Graph representation using NativeWind) */}
              <View className="h-44 flex-row items-end justify-between px-2 pt-6 bg-slate-50 rounded-xl border border-slate-100">
                {activeTelemetryTab === 'BP' && chartData.BP.map((d, index) => (
                  <View key={index} className="items-center flex-1 h-full justify-end">
                    {/* Systolic Bar */}
                    <View 
                      style={{ height: `${(d.systolic / 160) * 100}%` }} 
                      className="w-2.5 bg-teal-600 rounded-t-sm"
                    />
                    {/* Diastolic Bar */}
                    <View 
                      style={{ height: `${(d.diastolic / 160) * 100}%` }} 
                      className="w-2.5 bg-sky-400 rounded-t-sm mt-0.5"
                    />
                    <Text className="text-[9px] text-slate-400 mt-2 font-medium">{d.day}</Text>
                  </View>
                ))}

                {activeTelemetryTab === 'Glucose' && chartData.Glucose.map((d, index) => (
                  <View key={index} className="items-center flex-1 h-full justify-end">
                    <View 
                      style={{ height: `${(d.val / 200) * 100}%` }} 
                      className={`w-3.5 rounded-t-sm ${d.val > 140 ? 'bg-amber-500' : 'bg-teal-500'}`}
                    />
                    <Text className="text-[9px] text-slate-400 mt-2 font-medium">{d.day}</Text>
                  </View>
                ))}

                {activeTelemetryTab === 'HR' && chartData.HR.map((d, index) => (
                  <View key={index} className="items-center flex-1 h-full justify-end">
                    <View 
                      style={{ height: `${(d.val / 120) * 100}%` }} 
                      className="w-3 bg-rose-500 rounded-t-sm"
                    />
                    <Text className="text-[9px] text-slate-400 mt-2 font-medium">{d.day}</Text>
                  </View>
                ))}
              </View>

              {/* Legend / Metrics Footer */}
              <View className="flex-row justify-between items-center mt-4 border-t border-slate-100 pt-3">
                <Text className="text-slate-500 text-xs">{t('sponsor.averageStats')}:</Text>
                <View className="flex-row space-x-3">
                  {activeTelemetryTab === 'BP' && (
                    <>
                      <View className="flex-row items-center">
                        <View className="w-2.5 h-2.5 bg-teal-600 rounded-xs mr-1" />
                        <Text className="text-slate-700 text-xs font-semibold">Sys Avg: {averages.BP.systolic}</Text>
                      </View>
                      <View className="flex-row items-center">
                        <View className="w-2.5 h-2.5 bg-sky-400 rounded-xs mr-1" />
                        <Text className="text-slate-700 text-xs font-semibold">Dia Avg: {averages.BP.diastolic}</Text>
                      </View>
                    </>
                  )}
                  {activeTelemetryTab === 'Glucose' && (
                    <View className="flex-row items-center">
                      <Heart size={12} className="text-teal-500 mr-1" />
                      <Text className="text-slate-700 text-xs font-semibold">Avg: {averages.Glucose} mg/dL</Text>
                    </View>
                  )}
                  {activeTelemetryTab === 'HR' && (
                    <View className="flex-row items-center">
                      <Heart size={12} className="text-rose-500 mr-1" />
                      <Text className="text-slate-700 text-xs font-semibold">Avg: {averages.HR} bpm</Text>
                    </View>
                  )}
                </View>
              </View>
            </View>
          </>
        )}

        {/* Section 3: Payments Panel */}
        <View className="bg-white p-5 rounded-2xl shadow-sm border border-slate-100">
          <View className="flex-row items-center justify-between mb-4">
            <View className="flex-row items-center">
              <CreditCard size={22} className="text-teal-700 mr-2" />
              <Text className="text-slate-800 text-lg font-bold">{t('sponsor.paymentsTitle')}</Text>
            </View>
          </View>

          <View className="bg-slate-50 p-4 rounded-xl border border-slate-100 mb-4">
            <View className="flex-row justify-between items-center mb-1">
              <Text className="text-slate-700 font-bold text-base">{t('sponsor.planType')}</Text>
              <Text className="text-teal-700 font-bold text-base">{t('sponsor.amount').split(': ')[1]}</Text>
            </View>
            <Text className="text-slate-500 text-xs">{t('sponsor.nextBilling')}</Text>
          </View>

          <View className="flex-col sm:flex-row space-y-2 sm:space-y-0 sm:space-x-2">
            <TouchableOpacity 
              onPress={handlePayNow}
              className="flex-1 bg-teal-700 py-3.5 rounded-xl items-center shadow-sm active:bg-teal-800"
            >
              <Text className="text-white font-bold text-sm">Card Payment (USD/EUR)</Text>
            </TouchableOpacity>

            <TouchableOpacity 
              onPress={handlePayUPI}
              className="flex-1 bg-indigo-700 py-3.5 rounded-xl items-center shadow-sm active:bg-indigo-800"
            >
              <Text className="text-white font-bold text-sm">UPI Payment (INR)</Text>
            </TouchableOpacity>
          </View>

          {/* HIPAA & Security Disclaimer */}
          <View className="flex-row items-center justify-center space-x-1 mt-4">
            <ShieldCheck size={14} className="text-emerald-600" />
            <Text className="text-[10px] text-slate-400 font-semibold uppercase tracking-wide">
              {t('sponsor.billingSecured')}
            </Text>
          </View>
        </View>
      </View>
    </ScrollView>
  );
}
