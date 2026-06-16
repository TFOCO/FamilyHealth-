import React, { useState } from 'react';
import { SafeAreaView, View, Text, TouchableOpacity, StatusBar } from 'react-native';
import { StatusBar as ExpoStatusBar } from 'expo-status-bar';
import { useTranslation } from 'react-i18next';
import './src/i18n'; // Initialize i18n
import SponsorDashboard from './src/screens/SponsorDashboard';
import ParentDashboard from './src/screens/ParentDashboard';
import { Users, User } from 'lucide-react-native';

export default function App() {
  const { t } = useTranslation();
  const [role, setRole] = useState<'sponsor' | 'parent'>('sponsor');

  return (
    <SafeAreaView className="flex-1 bg-slate-50">
      <StatusBar barStyle="light-content" />
      
      {/* Dynamic View container */}
      <View className="flex-1">
        {role === 'sponsor' ? <SponsorDashboard /> : <ParentDashboard />}
      </View>

      {/* Role Navigation Bar (Bottom Tabs) */}
      <View className="flex-row bg-white border-t border-slate-200 py-3 px-6 justify-around shadow-lg">
        {/* Sponsor Tab */}
        <TouchableOpacity 
          onPress={() => setRole('sponsor')}
          className="items-center flex-1"
        >
          <Users size={22} className={role === 'sponsor' ? 'text-teal-700' : 'text-slate-400'} />
          <Text className={`text-xs mt-1 font-semibold ${
            role === 'sponsor' ? 'text-teal-700' : 'text-slate-400'
          }`}>
            {t('common.sponsor')}
          </Text>
        </TouchableOpacity>

        {/* Parent Tab */}
        <TouchableOpacity 
          onPress={() => setRole('parent')}
          className="items-center flex-1"
        >
          <User size={22} className={role === 'parent' ? 'text-emerald-700' : 'text-slate-400'} />
          <Text className={`text-xs mt-1 font-semibold ${
            role === 'parent' ? 'text-emerald-700' : 'text-slate-400'
          }`}>
            {t('common.parent')}
          </Text>
        </TouchableOpacity>
      </View>
      
      <ExpoStatusBar style="auto" />
    </SafeAreaView>
  );
}
