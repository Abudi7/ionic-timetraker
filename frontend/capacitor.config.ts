import type { CapacitorConfig } from '@capacitor/cli';

const config: CapacitorConfig = {
  appId: 'com.example.timetrac',       // خلي اسم مميز لتطبيقك
  appName: 'timetrac',
  webDir: 'www',

  server: {
    cleartext: true,
    allowNavigation: [
      'http://192.168.1.80:8087',
      'http://localhost:8087',
      'http://10.0.2.2:8087'
    ]
  },
  
  
  plugins: {
    CapacitorHttp: {
      enabled: true,
    },
  },

  ios: {
    contentInset: 'always'
  },

  android: {
    allowMixedContent: true
  }
};

export default config;
