import { useState, useEffect } from 'react';
import { GoogleLogin } from '@react-oauth/google';
import { jwtDecode } from 'jwt-decode';
import { fetchPantry, updatePantryItem, extractItems, ingestOrder } from './api';

function App() {
  const [pantry, setPantry] = useState([]);
  const [orders, setOrders] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState('inventory');
  const [searchTerm, setSearchTerm] = useState('');

  const [editingId, setEditingId] = useState(null);
  const [editValue, setEditValue] = useState('');
  const [user, setUser] = useState(null);
  const [extractionResult, setExtractionResult] = useState(null);
  const [isExtracting, setIsExtracting] = useState(false);

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
        <div className={`tab ${activeTab === 'history' ? 'active' : ''}`} onClick={() => setActiveTab('history')}>Order History</div>
        <div className={`tab ${activeTab === 'upload' ? 'active' : ''}`} onClick={() => setActiveTab('upload')}>Upload Invoice</div>
      </div>

      {activeTab === 'inventory' && (
        <section>
          <div className="search-container">
            <span style={{ position: 'absolute', left: '0.75rem', top: '0.55rem', color: 'var(--text-secondary)', fontSize: '0.8rem' }}>🔍</span>
            <input
              className="search-input"
              placeholder="Search pantry..."
              value={searchTerm}
              onChange={e => setSearchTerm(e.target.value)}
            />
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
    </div>
  );
}

export default App;
