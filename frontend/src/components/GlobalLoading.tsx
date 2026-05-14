import { useGlobalStore } from '@/stores/globalStore';

export default function GlobalLoading() {
  const loading = useGlobalStore((s) => s.loading);
  if (!loading) return null;
  return (
    <div
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        zIndex: 9999,
        height: 3,
        background: 'rgba(22, 119, 255, 0.15)',
        overflow: 'hidden',
      }}
    >
      <div
        style={{
          height: '100%',
          width: '40%',
          background: '#1677ff',
          borderRadius: 2,
          animation: 'global-loading-slide 1s infinite ease-in-out',
        }}
      />
      <style>{`
        @keyframes global-loading-slide {
          0% { transform: translateX(-100%); }
          100% { transform: translateX(250%); }
        }
      `}</style>
    </div>
  );
}
