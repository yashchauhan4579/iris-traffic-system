import React, { useState } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { Box, Lock, Activity } from 'lucide-react';
import { useAuth } from '../contexts/AuthContext';

export function LoginPage() {
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [error, setError] = useState('');
    const [isLoading, setIsLoading] = useState(false);
    const { login } = useAuth();
    const navigate = useNavigate();
    const location = useLocation();

    const from = location.state?.from?.pathname || '/';

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setError('');
        setIsLoading(true);

        try {
            await login({ username, password });
            navigate(from, { replace: true });
        } catch (err: any) {
            setError(err.message || 'Failed to login');
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <div className="min-h-screen w-full flex items-center justify-center bg-background relative overflow-hidden">
            {/* Background Effects */}
            <div className="absolute top-0 left-0 w-full h-full overflow-hidden z-0">
                <div className="absolute top-[-10%] left-[-10%] w-[40%] h-[40%] rounded-full bg-primary/20 blur-[120px]" />
                <div className="absolute bottom-[-10%] right-[-10%] w-[40%] h-[40%] rounded-full bg-blue-500/20 blur-[120px]" />
            </div>

            <div className="relative z-10 w-full max-w-md p-6">
                <div className="glass rounded-2xl p-8 shadow-2xl border border-white/10">
                    <div className="flex flex-col items-center mb-8">
                        <div className="w-16 h-16 bg-white rounded-2xl flex items-center justify-center mb-4 shadow-lg shadow-white/10">
                            <Activity className="w-10 h-10 text-black" />
                        </div>
                        <h1 className="text-2xl font-bold bg-clip-text text-transparent bg-gradient-to-r from-white to-white/70">
                            Welcome Back
                        </h1>
                        <p className="text-sm text-gray-400 mt-2">
                            Sign in to access the control center
                        </p>
                    </div>

                    <form onSubmit={handleSubmit} className="space-y-6">
                        {error && (
                            <div className="p-3 rounded-lg bg-red-500/10 border border-red-500/20 text-red-500 text-sm text-center">
                                {error}
                            </div>
                        )}

                        <div className="space-y-2">
                            <label className="text-sm font-medium text-gray-300 ml-1">Username</label>
                            <div className="relative group">
                                <Box className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400 group-focus-within:text-primary transition-colors" />
                                <input
                                    type="text"
                                    value={username}
                                    onChange={(e) => setUsername(e.target.value)}
                                    className="w-full bg-black/20 border border-white/10 rounded-xl px-10 py-3 text-white placeholder-gray-500 focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/50 transition-all hover:bg-black/30"
                                    placeholder="Enter your username"
                                    required
                                />
                            </div>
                        </div>

                        <div className="space-y-2">
                            <label className="text-sm font-medium text-gray-300 ml-1">Password</label>
                            <div className="relative group">
                                <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400 group-focus-within:text-primary transition-colors" />
                                <input
                                    type="password"
                                    value={password}
                                    onChange={(e) => setPassword(e.target.value)}
                                    className="w-full bg-black/20 border border-white/10 rounded-xl px-10 py-3 text-white placeholder-gray-500 focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/50 transition-all hover:bg-black/30"
                                    placeholder="••••••••••••"
                                    required
                                />
                            </div>
                        </div>

                        <button
                            type="submit"
                            disabled={isLoading}
                            className="w-full bg-white hover:bg-white/90 text-black font-medium py-3 rounded-xl transition-all duration-200 transform hover:scale-[1.02] active:scale-[0.98] disabled:opacity-50 disabled:cursor-not-allowed shadow-lg shadow-white/10"
                        >
                            {isLoading ? (
                                <span className="flex items-center justify-center gap-2">
                                    <div className="w-5 h-5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                                    Signing in...
                                </span>
                            ) : (
                                'Sign In'
                            )}
                        </button>
                    </form>

                    <div className="mt-8 text-center text-xs text-gray-500">
                        Protected by secure authentication system
                    </div>
                </div>
            </div>
        </div>
    );
}
