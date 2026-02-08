import { useState, useEffect } from 'react';
import { GoogleLogin } from '@react-oauth/google';
import { jwtDecode } from 'jwt-decode';
import { fetchPantry, updatePantryItem, deletePantryItem, extractItems, ingestOrder, suggestMealPersonalized, fetchGoals, createGoal, deleteGoal } from './api';

function App() {
  const [pantry, setPantry] = useState([]);
  const [orders, setOrders] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState('inventory');
  const [searchTerm, setSearchTerm] = useState('');

  const [editingId, setEditingId] = useState(null);
  const [editValue, setEditValue] = useState('');
  const [deletingItem, setDeletingItem] = useState(null);
  const [user, setUser] = useState(null);
  const [extractionResult, setExtractionResult] = useState(null);
  const [isExtracting, setIsExtracting] = useState(false);
  const [mealSuggestions, setMealSuggestions] = useState(null);
  const [isLoadingSuggestions, setIsLoadingSuggestions] = useState(false);
  const [goals, setGoals] = useState([]);
  const [showAddGoal, setShowAddGoal] = useState(false);
  const [newGoalTitle, setNewGoalTitle] = useState('');
  const [newGoalDescription, setNewGoalDescription] = useState('');

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

  const loadData = async () => {
    setLoading(true);
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
      const data = await fetchPantry();
      setPantry(data);
      setDeletingItem(null);
    } catch (err) {
      console.error("Failed to delete", err);
      alert("Failed to delete item");
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
        name: item.item.name,
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

          {loading ? (
            <div style={{ display: 'flex', justifyContent: 'center', padding: '4rem' }}><div className="loader"></div></div>
          ) : !user ? (
            <div className="glass-panel p-6 text-secondary" style={{ textAlign: 'center' }}>Please log in to view your pantry.</div>
          ) : filteredPantry.length === 0 ? (
            <div className="glass-panel p-6 text-secondary" style={{ textAlign: 'center' }}>No items found.</div>
          ) : (
            <div className="glass-panel p-6">
              <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                <thead>
                  <tr style={{ borderBottom: '2px solid var(--border-color)', textAlign: 'left' }}>
                    <th style={{ padding: '0.75rem 0', fontWeight: 600 }}>Item</th>
                    <th style={{ padding: '0.75rem 0', fontWeight: 600 }}>Stock</th>
                    <th style={{ padding: '0.75rem 0', fontWeight: 600 }}>Unit</th>
                    <th style={{ padding: '0.75rem 0', fontWeight: 600, textAlign: 'right' }}>Action</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredPantry.map(item => {
                    const isLow = item.effective_quantity < 2;
                    return (
                      <tr key={item.id} style={{ borderBottom: '1px solid var(--border-color)' }}>
                        <td style={{ padding: '0.75rem 0' }}>{item.item.name}</td>
                        <td style={{ padding: '0.75rem 0', fontWeight: 600, color: isLow ? 'var(--danger)' : 'inherit' }}>
                          {item.effective_quantity}
                        </td>
                        <td style={{ padding: '0.75rem 0' }}><span className="badge">{item.item.unit}</span></td>
                        <td style={{ padding: '0.75rem 0', textAlign: 'right' }}>
                          <button className="icon-btn" onClick={() => handleEdit(item)}>Update</button>
                          <button className="icon-btn" style={{ marginLeft: '0.5rem', color: 'var(--danger)' }} onClick={() => handleDelete(item)}>Delete</button>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
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
                      <div key={i} className="badge" style={{ background: 'transparent', border: '1px solid var(--border-color)' }}>
                        {item.item?.name || item.raw_name} × {item.quantity}
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

      {showAddGoal && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000 }}>
          <div className="glass-panel p-6" style={{ width: '400px' }}>
            <h2 className="mb-4">🎯 Add New Goal</h2>
            <input
              type="text"
              placeholder="Goal title (e.g., Lose 5kg in 30 days)"
              className="mb-4"
              style={{ width: '100%', padding: '0.75rem', border: '1px solid var(--border-color)', borderRadius: '8px' }}
              value={newGoalTitle}
              onChange={e => setNewGoalTitle(e.target.value)}
              autoFocus
            />
            <textarea
              placeholder="Description (optional)"
              className="mb-4"
              style={{ width: '100%', padding: '0.75rem', border: '1px solid var(--border-color)', borderRadius: '8px', minHeight: '80px', resize: 'vertical' }}
              value={newGoalDescription}
              onChange={e => setNewGoalDescription(e.target.value)}
            />
            <div className="flex-between">
              <button className="btn btn-secondary" onClick={() => { setShowAddGoal(false); setNewGoalTitle(''); setNewGoalDescription(''); }}>Cancel</button>
              <button className="btn" onClick={handleAddGoal}>Add Goal</button>
            </div>
          </div>
        </div>
      )}

      {mealSuggestions && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000 }}>
          <div className="glass-panel p-6" style={{ width: '90%', maxWidth: '900px', maxHeight: '85vh', overflow: 'auto' }}>
            <div className="flex-between mb-4">
              <h2 style={{ margin: 0 }}>✨ Meal Suggestions</h2>
              <button 
                onClick={() => setMealSuggestions(null)} 
                style={{ background: 'transparent', border: 'none', fontSize: '1.5rem', cursor: 'pointer', color: 'var(--text-secondary)' }}
              >
                ×
              </button>
            </div>
            {(() => {
              try {
                const data = typeof mealSuggestions === 'string' ? JSON.parse(mealSuggestions) : mealSuggestions;
                return (
                  <>
                    <div style={{ background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)', color: 'white', padding: '1rem', borderRadius: '8px', marginBottom: '1rem' }}>
                      <div style={{ fontSize: '0.85rem', opacity: 0.9 }}>🎯 Your Goal</div>
                      <div style={{ fontSize: '1.1rem', fontWeight: 600 }}>{data.goal}</div>
                      <div style={{ fontSize: '0.85rem', marginTop: '0.25rem', opacity: 0.9 }}>Suggested {data.meal_type} options</div>
                    </div>
                    <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.875rem' }}>
                      <thead>
                        <tr style={{ background: 'var(--bg-secondary)', textAlign: 'left' }}>
                          <th style={{ padding: '0.75rem', borderBottom: '2px solid var(--border-color)', width: '15%' }}>Dish</th>
                          <th style={{ padding: '0.75rem', borderBottom: '2px solid var(--border-color)', width: '18%' }}>Ingredients</th>
                          <th style={{ padding: '0.75rem', borderBottom: '2px solid var(--border-color)', width: '30%' }}>How to Make</th>
                          <th style={{ padding: '0.75rem', borderBottom: '2px solid var(--border-color)', width: '8%' }}>Time</th>
                          <th style={{ padding: '0.75rem', borderBottom: '2px solid var(--border-color)', width: '8%' }}>Calories</th>
                          <th style={{ padding: '0.75rem', borderBottom: '2px solid var(--border-color)', width: '8%' }}>Protein</th>
                          <th style={{ padding: '0.75rem', borderBottom: '2px solid var(--border-color)', width: '13%' }}>Benefits</th>
                        </tr>
                      </thead>
                      <tbody>
                        {data.meals?.map((meal, idx) => (
                          <tr key={idx} style={{ borderBottom: '1px solid var(--border-color)' }}>
                            <td style={{ padding: '0.75rem', fontWeight: 600, verticalAlign: 'top' }}>{meal.name}</td>
                            <td style={{ padding: '0.75rem', verticalAlign: 'top' }}>
                              {Array.isArray(meal.ingredients) ? meal.ingredients.join(', ') : meal.ingredients}
                            </td>
                            <td style={{ padding: '0.75rem', verticalAlign: 'top', fontSize: '0.8rem' }}>{meal.instructions}</td>
                            <td style={{ padding: '0.75rem', verticalAlign: 'top', textAlign: 'center' }}>
                              <span className="badge">{meal.prep_time}</span>
                            </td>
                            <td style={{ padding: '0.75rem', verticalAlign: 'top', textAlign: 'center', fontWeight: 600, color: '#e67e22' }}>{meal.calories}</td>
                            <td style={{ padding: '0.75rem', verticalAlign: 'top', textAlign: 'center', fontWeight: 600, color: '#27ae60' }}>{meal.protein}</td>
                            <td style={{ padding: '0.75rem', verticalAlign: 'top', fontSize: '0.8rem', color: 'var(--text-secondary)' }}>{meal.benefits}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </>
                );
              } catch (e) {
                return (
                  <div style={{ whiteSpace: 'pre-wrap', lineHeight: 1.6 }}>
                    {mealSuggestions}
                  </div>
                );
              }
            })()}
            <div style={{ marginTop: '1.5rem', textAlign: 'right' }}>
              <button className="btn" onClick={() => setMealSuggestions(null)}>Close</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default App;
