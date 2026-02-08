import { useState, useEffect } from 'react';
import { GoogleLogin } from '@react-oauth/google';
import { jwtDecode } from 'jwt-decode';
import {
  fetchPantry,
  updatePantryItem,
  deletePantryItem,
  bulkDeletePantryItems,
  extractItems,
  ingestOrder,
  suggestMealPersonalized,
  fetchGoals,
  createGoal,
  deleteGoal,
  logMeal,
  fetchMealHistory,
  deleteMealLog
} from './api';

function App() {
  const [pantry, setPantry] = useState([]);
  const [orders, setOrders] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState('inventory');
  const [searchTerm, setSearchTerm] = useState('');

  const [editingId, setEditingId] = useState(null);
  const [editValue, setEditValue] = useState('');
  const [deletingItem, setDeletingItem] = useState(null);
  const [selectedIds, setSelectedIds] = useState(new Set());
  const [showBulkDeleteConfirm, setShowBulkDeleteConfirm] = useState(false);
  const [user, setUser] = useState(null);
  const [extractionResult, setExtractionResult] = useState(null);
  const [isExtracting, setIsExtracting] = useState(false);
  const [mealSuggestions, setMealSuggestions] = useState(null);
  const [isLoadingSuggestions, setIsLoadingSuggestions] = useState(false);
  const [goals, setGoals] = useState([]);
  const [showAddGoal, setShowAddGoal] = useState(false);
  const [newGoalTitle, setNewGoalTitle] = useState('');
  const [newGoalDescription, setNewGoalDescription] = useState('');
  const [viewingNutritionItem, setViewingNutritionItem] = useState(null);
  const [selectedMeal, setSelectedMeal] = useState(null);
  const [isLoggingMeal, setIsLoggingMeal] = useState(false);
  const [mealHistory, setMealHistory] = useState([]);

  useEffect(() => {
    const token = localStorage.getItem('token');
    if (token) {
      try {
        if (token.includes('.')) {
          const decoded = jwtDecode(token);
          setUser(decoded);
        } else {
          // Fallback for dev IDs like "1"
          setUser({ sub: token, name: `User ${token}` });
        }
      } catch (err) {
        console.error("Invalid token", err);
        // If it's not a JWT and not a simple ID, maybe it's just garbage
        if (token.length > 20) {
          localStorage.removeItem('token');
          setUser(null);
        } else {
          setUser({ sub: token, name: `User ${token}` });
        }
      }
    }
  }, []);

  useEffect(() => {
    if (user) {
      loadData();
    } else {
      setLoading(false);
    }
  }, [user, activeTab]);

  // SSE: Listen for real-time nutrition updates
  useEffect(() => {
    if (!user) return;

    const eventSource = new EventSource('http://localhost:8080/sse/nutrition');

    eventSource.addEventListener('nutrition_update', (event) => {
      try {
        const update = JSON.parse(event.data);
        console.log('Received nutrition update:', update);

        // Update pantry items with new nutrition data
        setPantry(prev => prev.map(item => {
          if (item.item.id === update.item_id) {
            return {
              ...item,
              item: {
                ...item.item,
                calories: update.calories,
                protein: update.protein,
                carbs: update.carbs,
                fat: update.fat,
                fiber: update.fiber,
                nutrition_verified: update.verified
              }
            };
          }
          return item;
        }));
      } catch (err) {
        console.error('Failed to parse nutrition update:', err);
      }
    });

    eventSource.addEventListener('connected', () => {
      console.log('SSE connected for nutrition updates');
    });

    eventSource.onerror = (err) => {
      console.error('SSE error:', err);
    };

    return () => {
      eventSource.close();
    };
  }, [user]);

  const loadData = async () => {
    setLoading(true);
    setSelectedIds(new Set());
    try {
      if (activeTab === 'inventory') {
        const data = await fetchPantry();
        setPantry(data);
        const goalsData = await fetchGoals();
        setGoals(goalsData || []);
      } else if (activeTab === 'goals') {
        const goalsData = await fetchGoals();
        setGoals(goalsData || []);
      } else if (activeTab === 'history') {
        const res = await fetch('http://localhost:8080/orders', {
          headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` }
        });
        const data = await res.json();
        setOrders(data);
      }
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleEdit = (item) => {
    setEditingId(item.id);
    setEditValue(item.effective_quantity);
  };

  const handleSave = async (id) => {
    const val = parseFloat(editValue);
    if (isNaN(val)) return;
    setEditingId(null);
    try {
      const itemToUpdate = pantry.find(p => p.id === id);
      await updatePantryItem(itemToUpdate.item_id, val);
      const data = await fetchPantry();
      setPantry(data);
    } catch (err) {
      console.error("Failed to save", err);
    }
  };

  const handleDelete = (item) => {
    setDeletingItem(item);
  };

  const handleConfirmDelete = async () => {
    if (!deletingItem) return;
    try {
      await deletePantryItem(deletingItem.item_id);
      loadData();
      setDeletingItem(null);
    } catch (err) {
      console.error("Failed to delete", err);
      alert("Failed to delete item");
    }
  };

  const handleSelectItem = (id) => {
    const next = new Set(selectedIds);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setSelectedIds(next);
  };

  const handleSelectAll = (e) => {
    if (e.target.checked) {
      setSelectedIds(new Set(filteredPantry.map(i => i.item_id)));
    } else {
      setSelectedIds(new Set());
    }
  };

  const handleBulkDelete = () => {
    if (selectedIds.size > 0) {
      setShowBulkDeleteConfirm(true);
    }
  };

  const handleConfirmBulkDelete = async () => {
    try {
      await bulkDeletePantryItems(Array.from(selectedIds));
      loadData();
      setShowBulkDeleteConfirm(false);
    } catch (err) {
      console.error("Failed to bulk delete", err);
      alert("Failed to delete selected items");
    }
  };

  const handleFileUpload = async (e) => {
    const file = e.target.files[0];
    if (!file) return;
    setIsExtracting(true);
    try {
      const result = await extractItems(file);
      setExtractionResult(result);
    } catch (err) {
      alert("Failed to extract items from invoice.");
    } finally {
      setIsExtracting(false);
    }
  };

  const handleConfirmIngest = async () => {
    if (!extractionResult) return;
    try {
      const orderData = {
        user_id: user?.sub || user?.id || "1",
        external_order_id: `INV_${Date.now()}`,
        provider: extractionResult.provider || "unknown",
        order_date: new Date().toISOString(),
        items: extractionResult.items.map(item => ({
          raw_name: item.name,
          quantity: item.count * item.unit_value,
          unit: item.unit
        }))
      };

      await ingestOrder(orderData);
      setExtractionResult(null);
      loadData();
      setActiveTab('inventory');
    } catch (err) {
      console.error("Ingestion failed", err);
      alert("Failed to ingest order.");
    }
  };

  const filteredPantry = pantry.filter(item =>
    item.item.name.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const getTimeOfDay = () => {
    const hour = new Date().getHours();
    if (hour >= 5 && hour < 12) return 'morning';
    if (hour >= 12 && hour < 17) return 'afternoon';
    if (hour >= 17 && hour < 21) return 'evening';
    return 'night';
  };

  const handleSuggestMeal = async () => {
    if (pantry.length === 0) {
      alert('No items in pantry to suggest meals from');
      return;
    }
    setIsLoadingSuggestions(true);
    try {
      const inventory = pantry.map(item => ({
        name: item.item.ingredient?.name || item.item.name,
        quantity: item.effective_quantity,
        unit: item.item.unit
      }));
      const goalsForLLM = goals.map(g => ({
        title: g.title,
        description: g.description || ''
      }));
      const result = await suggestMealPersonalized(inventory, goalsForLLM, getTimeOfDay());
      setMealSuggestions(result.suggestions);
    } catch (err) {
      console.error('Failed to get meal suggestions', err);
      alert('Failed to get meal suggestions: ' + err.message);
    } finally {
      setIsLoadingSuggestions(false);
    }
  };

  const handleAddGoal = async () => {
    if (!newGoalTitle.trim()) {
      alert('Please enter a goal title');
      return;
    }
    try {
      await createGoal(newGoalTitle, newGoalDescription);
      const goalsData = await fetchGoals();
      setGoals(goalsData || []);
      setNewGoalTitle('');
      setNewGoalDescription('');
      setShowAddGoal(false);
    } catch (err) {
      console.error('Failed to create goal', err);
      alert('Failed to create goal');
    }
  };

  const handleDeleteGoal = async (goalId) => {
    try {
      await deleteGoal(goalId);
      setGoals(goals.filter(g => g.id !== goalId));
    } catch (err) {
      console.error('Failed to delete goal', err);
      alert('Failed to delete goal');
    }
  };

  return (
    <div className="App">
      <header className="flex-between mb-8">
        <h1>Pantry.</h1>
        <div style={{ display: 'flex', gap: '1rem', alignItems: 'center' }}>
          {user ? (
            <>
              <div className="badge" style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', background: 'white', border: '1px solid var(--border-color)' }}>
                {user.picture && <img src={user.picture} alt="avatar" style={{ width: 16, height: 16, borderRadius: '50%' }} />}
                <span>{user.name}</span>
              </div>
              <button className="btn btn-secondary" style={{ padding: '0.3rem 0.6rem', fontSize: '0.75rem' }} onClick={() => { setUser(null); localStorage.removeItem('token'); }}>
                Logout
              </button>
            </>
          ) : (
            <GoogleLogin
              onSuccess={cred => {
                const decoded = jwtDecode(cred.credential);
                setUser(decoded);
                localStorage.setItem('token', cred.credential);
              }}
              onError={() => console.log('Login Failed')}
            />
          )}
        </div>
      </header>

      <div className="tabs">
        <div className={`tab ${activeTab === 'inventory' ? 'active' : ''}`} onClick={() => setActiveTab('inventory')}>Inventory</div>
        <div className={`tab ${activeTab === 'goals' ? 'active' : ''}`} onClick={() => setActiveTab('goals')}>My Goals</div>
        <div className={`tab ${activeTab === 'meals' ? 'active' : ''}`} onClick={() => { setActiveTab('meals'); fetchMealHistory().then(setMealHistory).catch(console.error); }}>🍽️ Meals</div>
        <div className={`tab ${activeTab === 'nutrition' ? 'active' : ''}`} onClick={() => setActiveTab('nutrition')}>🔥 Nutrition</div>
        <div className={`tab ${activeTab === 'history' ? 'active' : ''}`} onClick={() => setActiveTab('history')}>Order History</div>
        <div className={`tab ${activeTab === 'upload' ? 'active' : ''}`} onClick={() => setActiveTab('upload')}>Upload Invoice</div>
      </div>

      {activeTab === 'inventory' && (
        <section>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '1rem', marginBottom: '1rem' }}>
            <div className="search-container" style={{ marginBottom: 0 }}>
              <span style={{ position: 'absolute', left: '0.75rem', top: '0.55rem', color: 'var(--text-secondary)', fontSize: '0.8rem' }}>🔍</span>
              <input
                className="search-input"
                placeholder="Search pantry..."
                value={searchTerm}
                onChange={e => setSearchTerm(e.target.value)}
              />
            </div>
            {user && pantry.length > 0 && (
              <button
                className="btn"
                onClick={handleSuggestMeal}
                disabled={isLoadingSuggestions}
                style={{ whiteSpace: 'nowrap' }}
              >
                {isLoadingSuggestions ? '✨ Thinking...' : '✨ Suggest a Meal'}
              </button>
            )}
          </div>

          {mealSuggestions && (
            <div className="glass-panel p-6 mb-6" style={{ background: 'linear-gradient(135deg, var(--card-bg) 0%, var(--bg-color) 100%)', borderLeft: '4px solid #8b5cf6' }}>
              <div className="flex-between mb-4">
                <h2 style={{ margin: 0, display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  ✨ Personalized Suggestions
                </h2>
                <button className="icon-btn" onClick={() => setMealSuggestions(null)}>Clear</button>
              </div>

              {(() => {
                try {
                  const data = typeof mealSuggestions === 'string' ? JSON.parse(mealSuggestions) : mealSuggestions;
                  return (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
                      <div className="text-secondary mb-2">
                        Suggestions for your <strong>{data.meal_type}</strong> based on your goal of <strong>{data.goal}</strong>:
                      </div>
                      <div className="grid-cols-auto">
                        {data.meals.map((meal, i) => (
                          <div key={i} className="glass-panel p-4" style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
                            <h3 style={{ margin: '0 0 0.5rem 0', fontSize: '1rem' }}>{meal.name}</h3>
                            <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', marginBottom: '0.75rem' }}>
                              <span className="badge" style={{ background: '#ede9fe', color: '#6d28d9' }}>🔥 {meal.calories} kcal</span>
                              <span className="badge" style={{ background: '#dcfce7', color: '#15803d' }}>💪 {meal.protein} protein</span>
                              <span className="badge">⏱️ {meal.prep_time}</span>
                            </div>
                            <div className="text-secondary mb-3" style={{ fontSize: '0.8rem', flex: 1 }}>
                              <strong>Benefits:</strong> {meal.benefits}
                            </div>
                            <button
                              className="btn"
                              style={{ width: '100%', marginTop: 'auto' }}
                              onClick={() => {
                                console.log('View Details clicked, meal:', meal);
                                setSelectedMeal(meal);
                              }}
                            >
                              View Details →
                            </button>
                          </div>
                        ))}
                      </div>
                    </div>
                  );
                } catch (e) {
                  return <pre style={{ whiteSpace: 'pre-wrap', fontSize: '0.875rem' }}>{mealSuggestions}</pre>;
                }
              })()}
            </div>
          )}

          {loading ? (
            <div style={{ display: 'flex', justifyContent: 'center', padding: '4rem' }}><div className="loader"></div></div>
          ) : !user ? (
            <div className="glass-panel p-6 text-secondary" style={{ textAlign: 'center' }}>Please log in to view your pantry.</div>
          ) : filteredPantry.length === 0 ? (
            <div className="glass-panel p-6 text-secondary" style={{ textAlign: 'center' }}>No items found.</div>
          ) : (
            <>
              {selectedIds.size > 0 && (
                <div className="glass-panel p-4 mb-4 flex-between" style={{ background: 'var(--danger-light)', borderColor: 'var(--danger)', borderLeftWidth: '4px' }}>
                  <span style={{ fontWeight: 600 }}>{selectedIds.size} items selected</span>
                  <button className="btn" style={{ background: 'var(--danger)' }} onClick={handleBulkDelete}>Delete Selected</button>
                </div>
              )}
              <div className="glass-panel p-6">
                <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                  <thead>
                    <tr style={{ borderBottom: '2px solid var(--border-color)', textAlign: 'left' }}>
                      <th style={{ padding: '0.75rem 0', width: '40px' }}>
                        <input
                          type="checkbox"
                          checked={selectedIds.size === filteredPantry.length && filteredPantry.length > 0}
                          onChange={handleSelectAll}
                        />
                      </th>
                      <th style={{ padding: '0.75rem 0', fontWeight: 600 }}>Item</th>
                      <th style={{ padding: '0.75rem 0', fontWeight: 600 }}>Stock</th>
                      <th style={{ padding: '0.75rem 0', fontWeight: 600 }}>Unit</th>
                      <th style={{ padding: '0.75rem 0', fontWeight: 600, textAlign: 'right' }}>Action</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredPantry.map(item => {
                      const isLow = item.effective_quantity < 2;
                      const isSelected = selectedIds.has(item.item_id);
                      return (
                        <tr key={item.id} style={{ borderBottom: '1px solid var(--border-color)', background: isSelected ? 'rgba(0,0,0,0.02)' : 'transparent' }}>
                          <td style={{ padding: '0.75rem 0' }}>
                            <input
                              type="checkbox"
                              checked={isSelected}
                              onChange={() => handleSelectItem(item.item_id)}
                            />
                          </td>
                          <td style={{ padding: '0.75rem 0' }}>
                            <div style={{ fontWeight: 600 }}>{item.ingredient?.name || item.item.name}</div>
                            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', display: 'flex', alignItems: 'center' }}>
                              {item.item.brand?.name && (
                                <span className="badge" style={{ padding: '0.1rem 0.3rem', marginRight: '0.4rem', background: '#fef3c7', color: '#92400e', fontSize: '0.65rem' }}>
                                  {item.item.brand.name}
                                </span>
                              )}
                              {item.item.product_name || item.item.name}
                            </div>
                            {item.item.calories > 0 && (
                              <div style={{ fontSize: '0.65rem', color: 'var(--text-secondary)', marginTop: '0.2rem' }}>
                                <strong>🔥 {Math.round(item.item.calories)}</strong> {
                                  ['pc', 'pcs', 'unit', 'units', 'piece', 'pieces', 'pack', 'dozen'].includes(item.item.unit?.toLowerCase())
                                    ? `kcal/${item.item.unit.toLowerCase()}`
                                    : `kcal/100${item.item.unit === 'ml' ? 'ml' : 'g'}`
                                } | P: {item.item.protein?.toFixed(1)}g | C: {item.item.carbs?.toFixed(1)}g | F: {item.item.fat?.toFixed(1)}g
                                {!item.item.nutrition_verified && <span style={{ fontStyle: 'italic', marginLeft: '0.4rem', opacity: 0.7 }}>(Est.)</span>}
                              </div>
                            )}
                          </td>
                          <td style={{ padding: '0.75rem 0', fontWeight: 600, color: isLow ? 'var(--danger)' : 'inherit' }}>
                            {item.effective_quantity}
                          </td>
                          <td style={{ padding: '0.75rem 0' }}><span className="badge">{item.item.unit}</span></td>
                          <td style={{ padding: '0.75rem 0', textAlign: 'right' }}>
                            <button className="icon-btn" style={{ marginRight: '0.5rem', color: 'var(--accent-color)' }} onClick={() => setViewingNutritionItem(item.item)}>📊</button>
                            <button className="icon-btn" onClick={() => handleEdit(item)}>Update</button>
                            <button className="icon-btn" style={{ marginLeft: '0.5rem', color: 'var(--danger)' }} onClick={() => handleDelete(item)}>Delete</button>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </section>
      )}

      {activeTab === 'goals' && (
        <section>
          {!user ? (
            <div className="glass-panel p-6 text-secondary" style={{ textAlign: 'center' }}>Please log in to manage your goals.</div>
          ) : loading ? (
            <div style={{ display: 'flex', justifyContent: 'center', padding: '4rem' }}><div className="loader"></div></div>
          ) : (
            <>
              <div className="glass-panel p-6 mb-4">
                <div className="flex-between mb-4">
                  <h2 style={{ margin: 0 }}>🎯 My Health Goals</h2>
                  <button className="btn" onClick={() => setShowAddGoal(true)}>+ Add Goal</button>
                </div>
                {goals.length === 0 ? (
                  <p className="text-secondary">No goals set yet. Add a goal to get personalized meal suggestions!</p>
                ) : (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
                    {goals.map(goal => (
                      <div key={goal.id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', padding: '1rem', background: 'rgba(0,0,0,0.02)', borderRadius: '8px', border: '1px solid var(--border-color)' }}>
                        <div>
                          <div style={{ fontWeight: 600, marginBottom: '0.25rem' }}>{goal.title}</div>
                          {goal.description && <div className="text-secondary" style={{ fontSize: '0.875rem' }}>{goal.description}</div>}
                        </div>
                        <button
                          onClick={() => handleDeleteGoal(goal.id)}
                          style={{ background: 'transparent', border: 'none', color: 'var(--danger)', cursor: 'pointer', fontSize: '1.2rem' }}
                        >×</button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
              <div className="glass-panel p-6" style={{ background: 'linear-gradient(135deg, #f5f7fa 0%, #e4e8eb 100%)' }}>
                <h3 style={{ margin: '0 0 0.5rem 0' }}>💡 Goal Ideas</h3>
                <p className="text-secondary mb-4" style={{ fontSize: '0.875rem' }}>Click to add any of these common goals:</p>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem' }}>
                  {[
                    { title: 'Lose 5kg in 30 days', desc: 'Focus on low-calorie, high-protein meals' },
                    { title: 'Build muscle', desc: 'High protein diet with complex carbs' },
                    { title: 'Healthy pregnancy diet', desc: 'Nutrient-rich meals for expecting mothers' },
                    { title: 'Train for 10K run', desc: 'Endurance-focused nutrition plan' },
                    { title: 'Reduce sugar intake', desc: 'Cut down on processed sugars' },
                    { title: 'Heart-healthy eating', desc: 'Low sodium, good fats' },
                  ].map((preset, i) => (
                    <button
                      key={i}
                      className="btn btn-secondary"
                      style={{ fontSize: '0.8rem', padding: '0.4rem 0.8rem' }}
                      onClick={async () => {
                        try {
                          await createGoal(preset.title, preset.desc);
                          const goalsData = await fetchGoals();
                          setGoals(goalsData || []);
                        } catch (err) {
                          alert('Failed to add goal');
                        }
                      }}
                    >
                      {preset.title}
                    </button>
                  ))}
                </div>
              </div>
            </>
          )}
        </section>
      )}

      {activeTab === 'nutrition' && (
        <section>
          {!user ? (
            <div className="glass-panel p-6 text-secondary" style={{ textAlign: 'center' }}>Please log in to view nutrition insights.</div>
          ) : pantry.length === 0 ? (
            <div className="glass-panel p-6 text-secondary" style={{ textAlign: 'center' }}>No items in pantry to analyze.</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '2rem' }}>
              {/* Macro Summary Cards */}
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '1.5rem' }}>
                {[
                  { label: 'Total Stock Calories', value: pantry.reduce((acc, p) => acc + (p.item.calories * (p.effective_quantity * (p.item.unit === 'g' || p.item.unit === 'ml' ? 1 : 100)) / 100), 0), color: '#ef4444', icon: '🔥', unit: 'kcal' },
                  { label: 'Protein Potential', value: pantry.reduce((acc, p) => acc + (p.item.protein * (p.effective_quantity * (p.item.unit === 'g' || p.item.unit === 'ml' ? 1 : 100)) / 100), 0), color: '#8b5cf6', icon: '💪', unit: 'g' },
                  { label: 'Fiber Reserve', value: pantry.reduce((acc, p) => acc + (p.item.fiber * (p.effective_quantity * (p.item.unit === 'g' || p.item.unit === 'ml' ? 1 : 100)) / 100), 0), color: '#10b981', icon: '🥗', unit: 'g' },
                ].map((stat, i) => (
                  <div key={i} className="glass-panel p-6" style={{ borderBottom: `4px solid ${stat.color}` }}>
                    <div style={{ fontSize: '1.5rem', marginBottom: '0.5rem' }}>{stat.icon}</div>
                    <div style={{ fontSize: '1.75rem', fontWeight: 700, color: stat.color }}>{Math.round(stat.value).toLocaleString()}<span style={{ fontSize: '0.9rem', fontWeight: 500, marginLeft: '0.25rem' }}>{stat.unit}</span></div>
                    <div className="text-secondary" style={{ fontSize: '0.85rem', fontWeight: 500 }}>{stat.label} (total stock)</div>
                  </div>
                ))}
              </div>

              {/* Item Macro Leaderboard */}
              <div className="glass-panel p-6">
                <h3 className="mb-4">Top Protein Sources</h3>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
                  {[...pantry].sort((a, b) => b.item.protein - a.item.protein).slice(0, 5).map((p, i) => (
                    <div key={i} className="flex-between p-3" style={{ background: 'rgba(0,0,0,0.02)', borderRadius: '8px' }}>
                      <div>
                        <div style={{ fontWeight: 600 }}>{p.item.ingredient?.name || p.item.name}</div>
                        <div className="text-secondary" style={{ fontSize: '0.75rem' }}>{p.item.brand?.name}</div>
                      </div>
                      <div style={{ fontWeight: 700, color: '#8b5cf6' }}>{p.item.protein} g <span style={{ fontSize: '0.7rem', fontWeight: 400, color: 'var(--text-secondary)' }}>/ {['pc', 'pcs', 'unit', 'units', 'piece', 'pieces', 'pack', 'dozen'].includes(p.item.unit?.toLowerCase()) ? p.item.unit.toLowerCase() : `100${p.item.unit === 'ml' ? 'ml' : 'g'}`}</span></div>
                    </div>
                  ))}
                </div>
              </div>

              <div className="glass-panel p-6" style={{ background: 'linear-gradient(135deg, #eff6ff 0%, #dbeafe 100%)', border: 'none' }}>
                <h3 style={{ color: '#1e40af', margin: '0 0 0.5rem 0' }}>💡 Health Tip</h3>
                <p style={{ color: '#1e3a8a', fontSize: '0.9rem', margin: 0 }}>
                  Based on your pantry, you have a solid supply of <strong>{pantry.reduce((a, b) => a.item.protein > b.item.protein ? a : b).item.ingredient?.name}</strong>.
                  Try pairing it with complex carbs from your stock to maintain steady energy levels throughout the day!
                </p>
              </div>
            </div>
          )}
        </section>
      )}

      {activeTab === 'meals' && (
        <section>
          <h2 style={{ marginBottom: '1rem' }}>🍽️ Meal History</h2>
          {!user ? (
            <div className="glass-panel p-6 text-secondary" style={{ textAlign: 'center' }}>Please log in to view meal history.</div>
          ) : mealHistory.length === 0 ? (
            <div className="glass-panel p-6 text-secondary" style={{ textAlign: 'center' }}>
              No meals logged yet. Log a meal from the suggested recipes!
            </div>
          ) : (
            <div className="grid-cols-auto">
              {mealHistory.map((meal) => (
                <div key={meal.id} className="glass-panel p-4" style={{ position: 'relative' }}>
                  <button
                    className="icon-btn"
                    style={{ position: 'absolute', top: '0.5rem', right: '0.5rem', color: 'var(--danger)' }}
                    onClick={async () => {
                      if (confirm('Delete this meal and restore ingredients to pantry?')) {
                        try {
                          await deleteMealLog(meal.id);
                          setMealHistory(mealHistory.filter(m => m.id !== meal.id));
                          const data = await fetchPantry();
                          setPantry(data);
                          alert('Meal deleted and pantry restored!');
                        } catch (err) {
                          console.error('Failed to delete meal', err);
                          alert('Failed to delete meal');
                        }
                      }
                    }}
                  >
                    🗑️
                  </button>
                  <h3 style={{ margin: '0 0 0.5rem 0', fontSize: '1rem', paddingRight: '2rem' }}>{meal.name}</h3>
                  <div className="text-secondary" style={{ fontSize: '0.75rem', marginBottom: '0.75rem' }}>
                    {new Date(meal.logged_at).toLocaleString()}
                  </div>
                  <div style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap', marginBottom: '0.75rem' }}>
                    <span className="badge" style={{ background: '#ede9fe', color: '#6d28d9' }}>🔥 {meal.calories?.toFixed(0) || 0} kcal</span>
                    <span className="badge" style={{ background: '#dcfce7', color: '#15803d' }}>💪 {meal.protein?.toFixed(1) || 0}g protein</span>
                    <span className="badge" style={{ background: '#fee2e2', color: '#dc2626' }}>🍞 {meal.carbs?.toFixed(1) || 0}g carbs</span>
                    <span className="badge" style={{ background: '#fef3c7', color: '#d97706' }}>🧈 {meal.fat?.toFixed(1) || 0}g fat</span>
                  </div>
                  <details style={{ fontSize: '0.8rem' }}>
                    <summary className="text-secondary" style={{ cursor: 'pointer' }}>View Ingredients</summary>
                    <ul style={{ margin: '0.5rem 0 0 0', paddingLeft: '1.2rem' }}>
                      {JSON.parse(meal.ingredients || '[]').map((ing, i) => (
                        <li key={i} className="text-secondary">{ing}</li>
                      ))}
                    </ul>
                  </details>
                </div>
              ))}
            </div>
          )}
        </section>
      )}

      {activeTab === 'history' && (
        <section>
          {!user ? (
            <div className="glass-panel p-6 text-secondary" style={{ textAlign: 'center' }}>Please log in to view order history.</div>
          ) : loading ? (
            <div style={{ display: 'flex', justifyContent: 'center', padding: '4rem' }}><div className="loader"></div></div>
          ) : orders.length === 0 ? (
            <div className="glass-panel p-6 text-secondary" style={{ textAlign: 'center' }}>No orders found.</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
              {orders.map(order => (
                <div key={order.id} className="glass-panel p-6">
                  <div className="flex-between mb-4">
                    <div>
                      <div style={{ fontWeight: 600 }}>{order.provider.toUpperCase()} #{order.external_order_id.slice(-6)}</div>
                      <div className="text-secondary">{new Date(order.order_date).toLocaleDateString()}</div>
                    </div>
                    <span className="badge">{order.items?.length || 0} Items</span>
                  </div>
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem' }}>
                    {order.items?.map((item, i) => (
                      <div key={i} className="badge" style={{ background: 'transparent', border: '1px solid var(--border-color)', display: 'flex', flexDirection: 'column', alignItems: 'flex-start', padding: '0.5rem 0.75rem', borderRadius: '8px' }}>
                        <div style={{ fontWeight: 600, fontSize: '0.85rem' }}>{item.item?.ingredient?.name || item.item?.name || item.raw_name}</div>
                        {(item.item?.brand?.name || item.item?.product_name) && (
                          <div style={{ fontSize: '0.7rem', color: 'var(--text-secondary)' }}>
                            {item.item?.brand?.name} {item.item?.product_name}
                          </div>
                        )}
                        <div style={{ fontSize: '0.75rem', marginTop: '0.25rem', fontWeight: 500 }}>Qty: {item.quantity}</div>
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>
      )}

      {activeTab === 'upload' && (
        <section>
          {!extractionResult ? (
            <div className="glass-panel p-6" style={{ textAlign: 'center', border: '2px dashed var(--border-color)', background: 'transparent' }}>
              <h2 className="mb-2">Invoice Upload</h2>
              <p className="text-secondary mb-6">Upload receipt to update inventory automatically.</p>
              <label className="btn" style={{ cursor: 'pointer' }}>
                {isExtracting ? <div className="loader" style={{ width: 14, height: 14 }}></div> : "Select File"}
                <input type="file" hidden onChange={handleFileUpload} accept=".pdf,.jpg,.png" />
              </label>
            </div>
          ) : (
            <div className="glass-panel p-6">
              <div className="flex-between mb-6">
                <h2 style={{ margin: 0 }}>Review {extractionResult.provider} Items</h2>
                <div style={{ display: 'flex', gap: '0.5rem' }}>
                  <button className="btn btn-secondary" onClick={() => setExtractionResult(null)}>Cancel</button>
                  <button className="btn" onClick={handleConfirmIngest}>Confirm</button>
                </div>
              </div>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.875rem' }}>
                <thead>
                  <tr style={{ borderBottom: '1px solid var(--border-color)', textAlign: 'left', color: 'var(--text-secondary)' }}>
                    <th style={{ padding: '0.5rem 0' }}>Name</th>
                    <th style={{ padding: '0.5rem 0' }}>Qty</th>
                    <th style={{ padding: '0.5rem 0' }}>Size</th>
                    <th style={{ padding: '0.5rem 0' }}>Unit</th>
                    <th style={{ padding: '0.5rem 0' }}></th>
                  </tr>
                </thead>
                <tbody>
                  {extractionResult.items.map((item, idx) => (
                    <tr key={idx} style={{ borderBottom: '1px solid var(--border-color)' }}>
                      <td style={{ padding: '0.5rem 0' }}>
                        {/* {console.log(extractionResult)} */}
                        <input style={{ background: 'transparent', border: 'none', width: '100%' }} value={item.name} onChange={e => { const n = { ...extractionResult }; n.items[idx].name = e.target.value; setExtractionResult(n); }} />
                      </td>
                      <td style={{ padding: '0.5rem 0' }}><input type="number" style={{ background: 'transparent', border: 'none', width: '40px' }} value={item.count} onChange={e => { const n = { ...extractionResult }; n.items[idx].count = parseFloat(e.target.value); setExtractionResult(n); }} /></td>
                      <td style={{ padding: '0.5rem 0' }}><input type="number" style={{ background: 'transparent', border: 'none', width: '40px' }} value={item.unit_value} onChange={e => { const n = { ...extractionResult }; n.items[idx].unit_value = parseFloat(e.target.value); setExtractionResult(n); }} /></td>
                      <td style={{ padding: '0.5rem 0' }}><input style={{ background: 'transparent', border: 'none', width: '40px' }} value={item.unit} onChange={e => { const n = { ...extractionResult }; n.items[idx].unit = e.target.value; setExtractionResult(n); }} /></td>
                      <td style={{ textAlign: 'right' }}><button style={{ background: 'transparent', border: 'none', color: 'var(--danger)', cursor: 'pointer' }} onClick={() => { const n = { ...extractionResult }; n.items = n.items.filter((_, i) => i !== idx); setExtractionResult(n); }}>✕</button></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      )}

      {editingId && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000 }}>
          <div className="glass-panel p-6" style={{ width: '280px' }}>
            <h2 className="mb-4">Update Stock</h2>
            <input
              type="number"
              className="mb-4"
              style={{ width: '100%', padding: '0.5rem', border: '1px solid var(--border-color)', borderRadius: '8px' }}
              value={editValue}
              onChange={e => setEditValue(e.target.value)}
              autoFocus
            />
            <div className="flex-between">
              <button className="btn btn-secondary" onClick={() => setEditingId(null)}>Cancel</button>
              <button className="btn" onClick={() => handleSave(editingId)}>Update</button>
            </div>
          </div>
        </div>
      )}

      {deletingItem && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000 }}>
          <div className="glass-panel p-6" style={{ width: '320px' }}>
            <h2 className="mb-4">Delete Item</h2>
            <p className="text-secondary mb-6">Are you sure you want to delete <strong>{deletingItem.item.name}</strong> from your pantry?</p>
            <div className="flex-between">
              <button className="btn btn-secondary" onClick={() => setDeletingItem(null)}>Cancel</button>
              <button className="btn" style={{ background: 'var(--danger)' }} onClick={handleConfirmDelete}>Delete</button>
            </div>
          </div>
        </div>
      )}

      {showBulkDeleteConfirm && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000 }}>
          <div className="glass-panel p-6" style={{ width: '320px' }}>
            <h2 className="mb-4">Delete Multiple Items</h2>
            <p className="text-secondary mb-6">Are you sure you want to delete <strong>{selectedIds.size}</strong> selected items from your pantry?</p>
            <div className="flex-between">
              <button className="btn btn-secondary" onClick={() => setShowBulkDeleteConfirm(false)}>Cancel</button>
              <button className="btn" style={{ background: 'var(--danger)' }} onClick={handleConfirmBulkDelete}>Delete All</button>
            </div>
          </div>
        </div>
      )}

      {showAddGoal && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000 }}>
          <div className="glass-panel p-6" style={{ width: '400px' }}>
            <h2 className="mb-4">🎯 Add Health Goal</h2>
            <div className="mb-4">
              <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 500, marginBottom: '0.25rem' }}>Goal Title</label>
              <input
                type="text"
                placeholder="e.g. Lose 5kg, Build Muscle"
                style={{ width: '100%', padding: '0.5rem', border: '1px solid var(--border-color)', borderRadius: '8px' }}
                value={newGoalTitle}
                onChange={e => setNewGoalTitle(e.target.value)}
                autoFocus
              />
            </div>
            <div className="mb-6">
              <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 500, marginBottom: '0.25rem' }}>Description (optional)</label>
              <textarea
                placeholder="Give more context to the AI..."
                style={{ width: '100%', padding: '0.5rem', border: '1px solid var(--border-color)', borderRadius: '8px', minHeight: '80px', fontFamily: 'inherit' }}
                value={newGoalDescription}
                onChange={e => setNewGoalDescription(e.target.value)}
              />
            </div>
            <div className="flex-between">
              <button className="btn btn-secondary" onClick={() => setShowAddGoal(false)}>Cancel</button>
              <button className="btn" onClick={handleAddGoal}>Save Goal</button>
            </div>
          </div>
        </div>
      )}
      {/* Nutrition Detail Modal */}
      {viewingNutritionItem && (
        <div className="modal-overlay" onClick={() => setViewingNutritionItem(null)}>
          <div className="glass-panel p-8" onClick={e => e.stopPropagation()} style={{ maxWidth: '400px', width: '90%', position: 'relative' }}>
            <button className="icon-btn" onClick={() => setViewingNutritionItem(null)} style={{ position: 'absolute', right: '1rem', top: '1rem', fontSize: '1.25rem' }}>×</button>
            <div className="mb-6">
              <h2 style={{ margin: '0 0 0.25rem 0' }}>{viewingNutritionItem.ingredient?.name || viewingNutritionItem.name}</h2>
              <div className="text-secondary" style={{ fontSize: '0.9rem' }}>
                {viewingNutritionItem.brand?.name && <span className="badge" style={{ padding: '0.1rem 0.3rem', marginRight: '0.5rem' }}>{viewingNutritionItem.brand.name}</span>}
                {viewingNutritionItem.product_name}
              </div>
            </div>

            <div style={{ background: 'rgba(0,0,0,0.02)', borderRadius: '12px', padding: '1.5rem', marginBottom: '1.5rem' }}>
              <div style={{ textAlign: 'center', marginBottom: '1.5rem' }}>
                <div style={{ fontSize: '2.5rem', fontWeight: 700, lineHeight: 1 }}>{Math.round(viewingNutritionItem.calories)}</div>
                <div className="text-secondary" style={{ fontSize: '0.8rem', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                  Calories / {['pc', 'pcs', 'unit', 'units', 'piece', 'pieces', 'pack', 'dozen'].includes(viewingNutritionItem.unit?.toLowerCase())
                    ? viewingNutritionItem.unit.toLowerCase()
                    : `100${viewingNutritionItem.unit === 'ml' ? 'ml' : 'g'}`}
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
                {[
                  { label: 'Protein', value: viewingNutritionItem.protein, color: '#8b5cf6', suffix: 'g' },
                  { label: 'Carbs', value: viewingNutritionItem.carbs, color: '#3b82f6', suffix: 'g' },
                  { label: 'Fats', value: viewingNutritionItem.fat, color: '#f59e0b', suffix: 'g' },
                  { label: 'Fiber', value: viewingNutritionItem.fiber, color: '#10b981', suffix: 'g' },
                ].map((macro, i) => (
                  <div key={i} style={{ padding: '0.75rem', background: 'white', borderRadius: '8px', border: '1px solid var(--border-color)' }}>
                    <div className="text-secondary" style={{ fontSize: '0.7rem', marginBottom: '0.2rem' }}>{macro.label}</div>
                    <div style={{ fontWeight: 600, fontSize: '1.1rem', color: macro.color }}>{macro.value?.toFixed(1)}{macro.suffix}</div>
                  </div>
                ))}
              </div>
            </div>

            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', textAlign: 'center' }}>
              {viewingNutritionItem.nutrition_verified ? (
                <span style={{ color: '#10b981' }}>● Verified Data</span>
              ) : (
                <span style={{ fontStyle: 'italic' }}>● Estimated by AI based on ingredient type</span>
              )}
            </div>

            <button className="btn btn-secondary w-full mt-6" onClick={() => setViewingNutritionItem(null)}>Close</button>
          </div>
        </div>
      )}

      {/* Meal Detail Modal */}
      {selectedMeal && (
        <div className="modal-overlay" onClick={() => setSelectedMeal(null)}>
          <div className="modal glass-panel p-6" onClick={(e) => e.stopPropagation()} style={{ maxWidth: '500px' }}>
            <div className="flex-between mb-4">
              <h2 style={{ margin: 0 }}>{selectedMeal.name}</h2>
              <button className="icon-btn" onClick={() => setSelectedMeal(null)}>✕</button>
            </div>

            <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', marginBottom: '1rem' }}>
              <span className="badge" style={{ background: '#ede9fe', color: '#6d28d9' }}>🔥 {selectedMeal.calories} kcal</span>
              <span className="badge" style={{ background: '#dcfce7', color: '#15803d' }}>💪 {selectedMeal.protein} protein</span>
              <span className="badge">⏱️ {selectedMeal.prep_time}</span>
            </div>

            <div style={{ marginBottom: '1rem' }}>
              <div style={{ fontWeight: 600, marginBottom: '0.5rem', color: 'var(--accent-color)' }}>Benefits</div>
              <div className="text-secondary">{selectedMeal.benefits}</div>
            </div>

            <div style={{ marginBottom: '1rem' }}>
              <div style={{ fontWeight: 600, marginBottom: '0.5rem', color: 'var(--accent-color)' }}>Ingredients</div>
              <ul style={{ margin: 0, paddingLeft: '1.2rem' }}>
                {selectedMeal.ingredients.map((ing, i) => (
                  <li key={i} className="text-secondary" style={{ marginBottom: '0.25rem' }}>{ing}</li>
                ))}
              </ul>
            </div>

            <div style={{ marginBottom: '1.5rem' }}>
              <div style={{ fontWeight: 600, marginBottom: '0.5rem', color: 'var(--accent-color)' }}>Instructions</div>
              <div className="text-secondary">{selectedMeal.instructions}</div>
            </div>

            <button
              className="btn w-full"
              style={{ background: 'linear-gradient(135deg, #10b981 0%, #059669 100%)', padding: '1rem' }}
              disabled={isLoggingMeal}
              onClick={async () => {
                setIsLoggingMeal(true);
                try {
                  const result = await logMeal(selectedMeal);
                  alert(`✅ Meal logged! Updated: ${result.updated_items?.join(', ') || 'No pantry items matched'}`);
                  setSelectedMeal(null);
                  // Refresh pantry to show updated quantities
                  const data = await fetchPantry();
                  setPantry(data);
                } catch (err) {
                  console.error('Failed to log meal', err);
                  alert('Failed to log meal: ' + err.message);
                } finally {
                  setIsLoggingMeal(false);
                }
              }}
            >
              {isLoggingMeal ? '⏳ Logging...' : '📋 Log This Meal'}
            </button>
            <div className="text-secondary" style={{ fontSize: '0.75rem', textAlign: 'center', marginTop: '0.5rem' }}>
              This will reduce your pantry quantities based on the ingredients used.
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default App;
