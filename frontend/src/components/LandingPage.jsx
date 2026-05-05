import React, { useState, useEffect } from 'react';

const ChatSimulator = () => {
  const [messages, setMessages] = useState([]);
  const [isTyping, setIsTyping] = useState(false);

  const script = [
    { role: 'user', content: 'Had 2 small aloo parathas with a bowl of dahi for breakfast.' },
    { role: 'bot', content: 'Perfect! 2 Aloo Parathas (approx 380 kcal) and Dahi (80 kcal). \n\nTotal: 460 kcal | 12g Protein. \n\nI’ve logged this for you. You have 1440 kcal left for the day. Should we aim for a high-protein lunch like Paneer or Dal?' }
  ];

  useEffect(() => {
    let timeout;
    const runScript = async () => {
      setMessages([]);
      
      // Step 1: User message
      timeout = setTimeout(() => {
        setMessages([script[0]]);
        
        // Step 2: Typing indicator
        timeout = setTimeout(() => {
          setIsTyping(true);
          
          // Step 3: Bot message
          timeout = setTimeout(() => {
            setIsTyping(false);
            setMessages(prev => [...prev, script[1]]);
            
            // Loop after delay
            timeout = setTimeout(runScript, 5000);
          }, 2000);
        }, 1000);
      }, 1000);
    };

    runScript();
    return () => clearTimeout(timeout);
  }, []);

  return (
    <div style={{
      maxWidth: '400px',
      width: '100%',
      background: '#e5ddd5', // WhatsApp background color
      borderRadius: '24px',
      overflow: 'hidden',
      boxShadow: '0 20px 40px rgba(0,0,0,0.15)',
      display: 'flex',
      flexDirection: 'column',
      height: '450px',
      fontFamily: 'Segoe UI, Roboto, Helvetica, Arial, sans-serif'
    }}>
      {/* WhatsApp Header */}
      <div style={{ background: '#075e54', color: 'white', padding: '1rem', display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
        <div style={{ width: '40px', height: '40px', background: '#ccc', borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '1.2rem' }}>🥗</div>
        <div>
          <div style={{ fontWeight: 600, fontSize: '0.95rem' }}>Pate Nutrition Coach</div>
          <div style={{ fontSize: '0.75rem', opacity: 0.8 }}>online</div>
        </div>
      </div>

      {/* Chat Area */}
      <div style={{ flex: 1, padding: '1.5rem', display: 'flex', flexDirection: 'column', gap: '1rem', backgroundImage: 'url("https://user-images.githubusercontent.com/15075759/28719144-86dc0f70-73b1-11e7-911d-60d70fcded21.png")', backgroundSize: 'contain' }}>
        {messages.map((msg, i) => (
          <div key={i} style={{
            alignSelf: msg.role === 'user' ? 'flex-end' : 'flex-start',
            background: msg.role === 'user' ? '#dcf8c6' : 'white',
            padding: '0.6rem 0.8rem',
            borderRadius: '8px',
            maxWidth: '85%',
            fontSize: '0.9rem',
            boxShadow: '0 1px 1px rgba(0,0,0,0.1)',
            whiteSpace: 'pre-wrap',
            position: 'relative',
            animation: 'fadeIn 0.3s ease-out forwards'
          }}>
            {msg.content}
            <div style={{ fontSize: '0.65rem', color: '#999', textAlign: 'right', marginTop: '4px' }}>12:45 PM</div>
          </div>
        ))}
        {isTyping && (
          <div style={{ alignSelf: 'flex-start', background: 'white', padding: '0.6rem 0.8rem', borderRadius: '8px', fontSize: '0.9rem', color: '#666' }}>
            typing...
          </div>
        )}
      </div>

      {/* Input Area */}
      <div style={{ padding: '0.75rem', background: '#f0f0f0', display: 'flex', gap: '0.5rem' }}>
        <div style={{ flex: 1, background: 'white', borderRadius: '20px', padding: '0.5rem 1rem', fontSize: '0.9rem', color: '#999' }}>Message</div>
        <div style={{ width: '35px', height: '35px', background: '#128c7e', borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'white' }}>➤</div>
      </div>

      <style>{`
        @keyframes fadeIn {
          from { opacity: 0; transform: translateY(10px); }
          to { opacity: 1; transform: translateY(0); }
        }
        @media (max-width: 768px) {
          .hero-title { font-size: 2.5rem !important; }
          .hero-section { padding: 120px 5% 60px !important; }
          .feature-grid { grid-template-columns: 1fr !important; }
          .flex-responsive { flex-direction: column !important; gap: 2rem !important; }
          .desktop-only { display: none !important; }
          .mobile-center { text-align: center !important; align-items: center !important; }
        }
      `}</style>
    </div>
  );
};

const LandingPage = ({ onLoginClick }) => {
  return (
    <div style={{ 
      fontFamily: "'Outfit', 'Inter', sans-serif", 
      color: '#1a1a1a', 
      backgroundColor: '#ffffff',
      overflowX: 'hidden'
    }}>
      {/* Navbar */}
      <nav style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        padding: '1.5rem 5%',
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        backgroundColor: 'rgba(255, 255, 255, 0.8)',
        backdropFilter: 'blur(10px)',
        zIndex: 1000,
        borderBottom: '1px solid #f0f0f0'
      }}>
        <div style={{ fontSize: '1.5rem', fontWeight: 800, color: '#7c3aed', letterSpacing: '-0.02em' }}>
          Pate<span style={{ color: '#10b981' }}>Project</span>
        </div>
        <button 
          onClick={onLoginClick}
          style={{
            padding: '0.6rem 1.5rem',
            borderRadius: '50px',
            background: 'linear-gradient(135deg, #7c3aed 0%, #4f46e5 100%)',
            color: 'white',
            border: 'none',
            fontWeight: 600,
            cursor: 'pointer',
            boxShadow: '0 4px 15px rgba(124, 58, 237, 0.3)',
            transition: 'transform 0.2s ease'
          }}
          onMouseOver={(e) => e.currentTarget.style.transform = 'translateY(-2px)'}
          onMouseOut={(e) => e.currentTarget.style.transform = 'translateY(0)'}
        >
          Get Started
        </button>
      </nav>

      {/* Hero Section */}
      <header className="hero-section" style={{
        padding: '160px 5% 80px',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        textAlign: 'center',
        background: 'radial-gradient(circle at top right, #f5f3ff 0%, #ffffff 50%)'
      }}>
        <div style={{
          display: 'inline-block',
          padding: '0.5rem 1.2rem',
          borderRadius: '50px',
          backgroundColor: '#ede9fe',
          color: '#7c3aed',
          fontSize: '0.8rem',
          fontWeight: 600,
          marginBottom: '1.5rem'
        }}>
          🇮🇳 India's #1 AI Nutrition Coach on WhatsApp
        </div>
        <h1 className="hero-title" style={{
          fontSize: 'clamp(2.5rem, 6vw, 4.5rem)',
          fontWeight: 900,
          lineHeight: 1.1,
          marginBottom: '1.5rem',
          letterSpacing: '-0.03em',
          maxWidth: '900px'
        }}>
          Ghar ka Khaana, <br/>
          <span style={{ 
            background: 'linear-gradient(135deg, #7c3aed 0%, #10b981 100%)', 
            WebkitBackgroundClip: 'text', 
            WebkitTextFillColor: 'transparent' 
          }}>Smartly Tracked.</span>
        </h1>
        <p style={{
          fontSize: '1.1rem',
          color: '#666',
          maxWidth: '600px',
          marginBottom: '2.5rem',
          lineHeight: 1.6
        }}>
          From Paneer Tikka to Poha, we understand Indian meals like no one else. Just text what you ate on WhatsApp. No app, no friction.
        </p>
        <div style={{ display: 'flex', gap: '1rem', flexWrap: 'wrap', justifyContent: 'center' }}>
          <button 
            onClick={onLoginClick}
            style={{
              padding: '1rem 2.5rem',
              borderRadius: '50px',
              backgroundColor: '#1a1a1a',
              color: 'white',
              fontSize: '1.1rem',
              fontWeight: 600,
              border: 'none',
              cursor: 'pointer',
              transition: 'all 0.2s ease'
            }}
          >
            Log Your First Meal
          </button>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', color: '#666', fontSize: '0.9rem' }}>
             <span style={{ color: '#25D366', fontSize: '1.5rem' }}>●</span> Instant setup via WhatsApp
          </div>
        </div>

        {/* Hero Image */}
        <div style={{ marginTop: '5rem', position: 'relative', maxWidth: '1000px', width: '100%' }}>
          <img 
            src="/landing/hero_indian.png" 
            alt="PateProject Indian Hero"
            style={{
              width: '100%',
              borderRadius: '24px',
              boxShadow: '0 30px 60px rgba(0,0,0,0.12)',
              border: '1px solid rgba(0,0,0,0.05)'
            }}
          />
        </div>
      </header>

      {/* Live Interaction Section */}
      <section style={{ padding: '60px 5%', backgroundColor: '#ffffff' }}>
        <div className="flex-responsive" style={{
          maxWidth: '1200px',
          margin: '0 auto',
          display: 'flex',
          flexDirection: 'row',
          alignItems: 'center',
          gap: '5rem',
          flexWrap: 'wrap'
        }}>
          <div className="mobile-center" style={{ flex: 1, minWidth: '300px' }}>
            <h2 style={{ fontSize: '2.5rem', fontWeight: 800, marginBottom: '1.5rem', letterSpacing: '-0.02em' }}>See it in action.</h2>
            <p style={{ fontSize: '1.1rem', color: '#666', lineHeight: 1.6, marginBottom: '2rem' }}>
              No more searching through 10,000 items in a database. Just tell us what you had in plain Hinglish or English.
            </p>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem', textAlign: 'left' }}>
              <div style={{ display: 'flex', gap: '1rem', alignItems: 'flex-start' }}>
                <div style={{ background: '#ede9fe', padding: '0.5rem', borderRadius: '12px' }}>🍲</div>
                <div>
                  <h4 style={{ margin: '0 0 0.25rem 0' }}>Understands Variety</h4>
                  <p style={{ margin: 0, color: '#666', fontSize: '0.95rem' }}>From Masala Dosa to Butter Chicken, we know the macros of your favorite dishes.</p>
                </div>
              </div>
              <div style={{ display: 'flex', gap: '1rem', alignItems: 'flex-start' }}>
                <div style={{ background: '#dcfce7', padding: '0.5rem', borderRadius: '12px' }}>📏</div>
                <div>
                  <h4 style={{ margin: '0 0 0.25rem 0' }}>Portion Precision</h4>
                  <p style={{ margin: 0, color: '#666', fontSize: '0.95rem' }}>Say "half a plate" or "2 medium rotis"—our AI estimates weight and calories accurately.</p>
                </div>
              </div>
            </div>
          </div>
          <div style={{ flex: 1, minWidth: '300px', display: 'flex', justifyContent: 'center' }}>
            <ChatSimulator />
          </div>
        </div>
      </section>

      {/* Features Section */}
      <section style={{ padding: '60px 5%', backgroundColor: '#f9fafb' }}>
        <div style={{ textAlign: 'center', marginBottom: '3rem' }}>
          <h2 style={{ fontSize: '2rem', fontWeight: 800, letterSpacing: '-0.02em' }}>The Smartest Way to Eat in India</h2>
        </div>

        <div className="feature-grid" style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))',
          gap: '1.5rem',
          maxWidth: '1200px',
          margin: '0 auto'
        }}>
          <div style={{ padding: '2.5rem', backgroundColor: 'white', borderRadius: '24px' }}>
            <div style={{ fontSize: '2.5rem', marginBottom: '1.5rem' }}>📲</div>
            <h3 style={{ fontSize: '1.5rem', fontWeight: 700, marginBottom: '1rem' }}>Zero App Fatigue</h3>
            <p style={{ color: '#666', lineHeight: 1.6 }}>Logging shouldn't feel like a chore. Send a quick WhatsApp text and get back to your meal. We handle the rest.</p>
          </div>
          <div style={{ padding: '2.5rem', backgroundColor: 'white', borderRadius: '24px' }}>
            <div style={{ fontSize: '2.5rem', marginBottom: '1.5rem' }}>🥘</div>
            <h3 style={{ fontSize: '1.5rem', fontWeight: 700, marginBottom: '1rem' }}>Indian Cuisine Expert</h3>
            <p style={{ color: '#666', lineHeight: 1.6 }}>Trained on regional Indian diets. Whether it's Kerala Sadhya or Punjabi Thali, we get the nutrition right.</p>
          </div>
          <div style={{ padding: '2.5rem', backgroundColor: 'white', borderRadius: '24px' }}>
            <div style={{ fontSize: '2.5rem', marginBottom: '1.5rem' }}>🤖</div>
            <h3 style={{ fontSize: '1.5rem', fontWeight: 700, marginBottom: '1rem' }}>Proactive Advice</h3>
            <p style={{ color: '#666', lineHeight: 1.6 }}>"You've had a lot of carbs today, Priya. For dinner, maybe try a high-protein Paneer Salad?"</p>
          </div>
        </div>
      </section>

      {/* Dashboard Highlight */}
      <section style={{ padding: '60px 5%', backgroundColor: 'white' }}>
        <div className="flex-responsive" style={{
          maxWidth: '1200px',
          margin: '0 auto',
          display: 'flex',
          flexDirection: 'row',
          alignItems: 'center',
          gap: '5rem',
          flexWrap: 'wrap-reverse'
        }}>
          <div style={{ flex: 1, minWidth: '300px' }}>
            <img 
              src="/landing/dashboard.png" 
              alt="Indian Nutrition Dashboard"
              style={{ width: '100%', borderRadius: '24px', boxShadow: '0 20px 40px rgba(0,0,0,0.1)' }}
            />
          </div>
          <div className="mobile-center" style={{ flex: 1, minWidth: '300px' }}>
            <h2 style={{ fontSize: '2.5rem', fontWeight: 800, marginBottom: '1.5rem', letterSpacing: '-0.02em' }}>Track like a Pro.</h2>
            <p style={{ fontSize: '1.1rem', color: '#666', lineHeight: 1.6, marginBottom: '2rem' }}>
              Beautiful charts that show your Protein, Carbs, and Fats. See how your favorite Indian meals fit into your long-term fitness goals.
            </p>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer style={{
        padding: '60px 5%',
        textAlign: 'center',
        background: 'linear-gradient(135deg, #7c3aed 0%, #4f46e5 100%)',
        color: 'white'
      }}>
        <h2 className="hero-title" style={{ fontSize: '2.5rem', fontWeight: 800, marginBottom: '1.5rem' }}>Start your 1-click tracking.</h2>
        <p style={{ fontSize: '1.1rem', opacity: 0.9, marginBottom: '2.5rem' }}>Join the community eating healthy, the Indian way.</p>
        <button 
          onClick={onLoginClick}
          style={{
            padding: '1rem 2.5rem',
            borderRadius: '50px',
            backgroundColor: 'white',
            color: '#7c3aed',
            fontSize: '1.1rem',
            fontWeight: 700,
            border: 'none',
            cursor: 'pointer'
          }}
        >
          Login with WhatsApp
        </button>
        <div style={{ marginTop: '5rem', opacity: 0.6, fontSize: '0.9rem' }}>
          &copy; 2026 PateProject. All rights reserved. <br/>
          <a href="/privacy.html" style={{ color: 'white', textDecoration: 'underline' }}>Privacy Policy</a>
        </div>
      </footer>
    </div>
  );
};

export default LandingPage;
