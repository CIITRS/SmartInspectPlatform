import React, { useEffect, useRef } from 'react';

interface BarcodeScannerProps {
  onScanResult: (code: string) => void;
  isEnabled: boolean;
}

// 简化版 BarcodeScanner 组件，先不使用 Quagga.js，避免导入错误
const BarcodeScanner: React.FC<BarcodeScannerProps> = ({ isEnabled }) => {
  const videoRef = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    let stream: MediaStream | null = null;
    
    // 只有当 isEnabled 为 true 时才启动摄像头
    if (isEnabled && navigator.mediaDevices && navigator.mediaDevices.getUserMedia) {
      navigator.mediaDevices.getUserMedia({ video: { facingMode: 'environment' } })
        .then((mediaStream) => {
          stream = mediaStream;
          if (videoRef.current) {
            videoRef.current.srcObject = mediaStream;
            videoRef.current.play();
          }
        })
        .catch((error) => {
          console.error('Error accessing camera:', error);
        });
    }

    // 清理函数
    return () => {
      if (stream) {
        stream.getTracks().forEach(track => {
          track.stop();
        });
      }
    };
  }, [isEnabled]);

  return (
    <div style={{ width: '100%', height: '100%', position: 'relative' }}>
      <video 
        ref={videoRef} 
        autoPlay
        playsInline
        muted
        style={{ width: '100%', height: '100%', objectFit: 'cover' }}
      >
        <track kind="captions" srcLang="zh" label="中文" />
      </video>
    </div>
  );
};

export default BarcodeScanner;