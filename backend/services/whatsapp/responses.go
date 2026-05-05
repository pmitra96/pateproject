package whatsapp

const (
	// System Status
	MsgThinking    = "Thinking... ⏳"
	MsgAnalyzing   = "👀 Analyzing your photo... give me a moment."
	MsgProcessing  = "Got it! Let me check that for you... ⏳"
	
	// Errors
	MsgErrorGeneral = "⚠️ I encountered a technical error. Please try again in a few minutes!"
	MsgErrorBrain   = "🧠 My AI brain is a bit overwhelmed right now. Please give me a few minutes to recover and try again!"
	MsgErrorLimit   = "🌙 You've reached your daily limit of AI messages. Please try again tomorrow!"
	MsgErrorEmpty   = "I heard you, but I'm not sure how to respond. Could you rephrase?"
	MsgErrorProfile = "I'm having trouble identifying your account."
	MsgErrorMedia   = "📸 I saw your photo, but I had trouble downloading it. Can you try again?"

	// Tool Feedback (Success)
	MsgGoalUpdated    = "🎯 *Goal Updated!* Your daily target is now %d calories."
	MsgProfileUpdated = "✅ *Profile Updated!* I've saved your details. This will help me give you much more accurate nutritional advice."
	MsgPantryUpdated  = "📦 *Pantry Updated!*"
	MsgMealLogged     = "✅ *Logged:*"
)
