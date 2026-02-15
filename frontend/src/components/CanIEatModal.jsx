import React, { useState } from 'react';

const CanIEatModal = ({
  isOpen,
  onClose,
  checkResult,
  foodName,
  onCheck,
  onReset,
  onLog,
  onLogSmall
}) => {
  const [query, setQuery] = useState('');
  const [isChecking, setIsChecking] = useState(false);

  if (!isOpen) return null;

  const handleCheck = async () => {
    if (!query.trim()) return;
    setIsChecking(true);
    try {
      await onCheck(query);
    } finally {
      setIsChecking(false);
    }
  };

  const handleReset = () => {
    setQuery('');
    onReset();
  };

  const handleKeyPress = (e) => {
    if (e.key === 'Enter' && !isChecking) {
      handleCheck();
    }
  };

  return (
    <div
      className="modal-overlay"
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        background: 'rgba(0,0,0,0.5)',
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        zIndex: 1000
      }}
      onClick={onClose}
    >
      <div
        className="glass-panel p-6"
        style={{
          background: 'white',
          maxWidth: '600px',
          width: '90%',
          maxHeight: '90vh',
          overflowY: 'auto',
          borderRadius: '8px'
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex justify-between items-center mb-4">
          <h2 style={{ margin: 0, fontSize: '1.5rem', fontWeight: 'bold' }}>Can I Eat This?</h2>
          <button
            onClick={onClose}
            style={{
              background: 'none',
              border: 'none',
              fontSize: '1.5rem',
              cursor: 'pointer',
              color: '#666'
            }}
          >
            ×
          </button>
        </div>

        {!checkResult ? (
          <div>
            <p style={{ marginBottom: '1rem', color: '#666' }}>
              Enter a food item to check if it fits within your daily goals.
            </p>
            <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem' }}>
              <input
                type="text"
                placeholder="e.g., 2 slices of pizza, 1 apple, chicken tikka masala"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyPress={handleKeyPress}
                disabled={isChecking}
                style={{
                  flex: 1,
                  padding: '0.75rem',
                  border: '1px solid #ddd',
                  borderRadius: '4px',
                  fontSize: '1rem'
                }}
              />
              <button
                onClick={handleCheck}
                disabled={!query.trim() || isChecking}
                className="btn btn-primary"
                style={{
                  padding: '0.75rem 1.5rem',
                  opacity: !query.trim() || isChecking ? 0.5 : 1
                }}
              >
                {isChecking ? 'Checking...' : 'Check'}
              </button>
            </div>
            <div style={{ fontSize: '0.875rem', color: '#888' }}>
              💡 Tip: Be specific about portions for better results
            </div>
          </div>
        ) : (
          <div>
            {/* Food Info */}
            <div
              style={{
                padding: '1rem',
                background: '#f9f9f9',
                borderRadius: '6px',
                marginBottom: '1rem'
              }}
            >
              <h3 style={{ margin: '0 0 0.5rem 0', fontSize: '1.25rem' }}>
                {checkResult.food?.name || foodName}
              </h3>
              {checkResult.food && (
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: '0.5rem' }}>
                  <div>
                    <div style={{ fontSize: '0.75rem', color: '#666' }}>Calories</div>
                    <div style={{ fontWeight: 'bold' }}>{Math.round(checkResult.food.calories)} kcal</div>
                  </div>
                  <div>
                    <div style={{ fontSize: '0.75rem', color: '#666' }}>Protein</div>
                    <div style={{ fontWeight: 'bold' }}>{Math.round(checkResult.food.protein)}g</div>
                  </div>
                  <div>
                    <div style={{ fontSize: '0.75rem', color: '#666' }}>Fat</div>
                    <div style={{ fontWeight: 'bold' }}>{Math.round(checkResult.food.fat)}g</div>
                  </div>
                  <div>
                    <div style={{ fontSize: '0.75rem', color: '#666' }}>Carbs</div>
                    <div style={{ fontWeight: 'bold' }}>{Math.round(checkResult.food.carbs)}g</div>
                  </div>
                </div>
              )}
            </div>

            {/* Decision */}
            <div
              style={{
                padding: '1rem',
                background: checkResult.permission_result?.allowed ? '#d4edda' : '#f8d7da',
                border: `2px solid ${checkResult.permission_result?.allowed ? '#28a745' : '#dc3545'}`,
                borderRadius: '6px',
                marginBottom: '1rem'
              }}
            >
              <div
                style={{
                  fontSize: '1.25rem',
                  fontWeight: 'bold',
                  color: checkResult.permission_result?.allowed ? '#155724' : '#721c24',
                  marginBottom: '0.5rem'
                }}
              >
                {checkResult.permission_result?.allowed ? '✅ Go Ahead!' : '⚠️ Hold On'}
              </div>
              <div style={{ color: checkResult.permission_result?.allowed ? '#155724' : '#721c24' }}>
                {checkResult.permission_result?.reason}
              </div>
            </div>

            {/* State Comparison */}
            {checkResult.current_state && checkResult.simulated_state && (
              <div style={{ marginBottom: '1rem' }}>
                <h4 style={{ margin: '0 0 0.75rem 0', fontSize: '1rem', color: '#666' }}>
                  Impact on Your Day
                </h4>
                <div
                  style={{
                    display: 'grid',
                    gridTemplateColumns: 'auto 1fr 1fr',
                    gap: '0.5rem',
                    fontSize: '0.875rem'
                  }}
                >
                  <div style={{ fontWeight: 'bold' }}></div>
                  <div style={{ fontWeight: 'bold', textAlign: 'center' }}>Current</div>
                  <div style={{ fontWeight: 'bold', textAlign: 'center' }}>After Eating</div>

                  <div>Calories</div>
                  <div style={{ textAlign: 'center' }}>
                    {Math.round(checkResult.current_state.remaining_calories)} kcal
                  </div>
                  <div
                    style={{
                      textAlign: 'center',
                      color: checkResult.simulated_state.remaining_calories < 0 ? '#dc3545' : 'inherit'
                    }}
                  >
                    {Math.round(checkResult.simulated_state.remaining_calories)} kcal
                  </div>

                  <div>Protein</div>
                  <div style={{ textAlign: 'center' }}>
                    {Math.round(checkResult.current_state.remaining_protein)}g
                  </div>
                  <div style={{ textAlign: 'center' }}>
                    {Math.round(checkResult.simulated_state.remaining_protein)}g
                  </div>

                  <div>Fat</div>
                  <div style={{ textAlign: 'center' }}>
                    {Math.round(checkResult.current_state.remaining_fat)}g
                  </div>
                  <div style={{ textAlign: 'center' }}>
                    {Math.round(checkResult.simulated_state.remaining_fat)}g
                  </div>

                  <div>Carbs</div>
                  <div style={{ textAlign: 'center' }}>
                    {Math.round(checkResult.current_state.remaining_carbs)}g
                  </div>
                  <div style={{ textAlign: 'center' }}>
                    {Math.round(checkResult.simulated_state.remaining_carbs)}g
                  </div>
                </div>
              </div>
            )}

            {/* Actions */}
            <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
              {checkResult.permission_result?.allowed && onLog && (
                <button
                  onClick={onLog}
                  className="btn btn-primary"
                  style={{ flex: 1, minWidth: '120px' }}
                >
                  Log This Meal
                </button>
              )}
              {!checkResult.permission_result?.allowed && onLogSmall && (
                <button
                  onClick={onLogSmall}
                  className="btn btn-secondary"
                  style={{ flex: 1, minWidth: '150px' }}
                >
                  Try Smaller Portion
                </button>
              )}
              <button
                onClick={handleReset}
                className="btn btn-secondary"
                style={{ flex: 1, minWidth: '120px' }}
              >
                Check Another
              </button>
              <button
                onClick={onClose}
                className="btn btn-secondary"
                style={{ minWidth: '80px' }}
              >
                Close
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default CanIEatModal;
