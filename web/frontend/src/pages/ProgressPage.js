import React, { useState, useEffect, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Box,
  Button,
  Container,
  Typography,
  LinearProgress,
  Paper,
  Grid,
  Card,
  CardContent,
  Divider,
  CircularProgress,
  Alert,
} from '@mui/material';
import AssessmentIcon from '@mui/icons-material/Assessment';
import GlassCard from '../components/GlassCard';
import { PieChart, Pie, Cell, ResponsiveContainer, Legend, Tooltip } from 'recharts';
import axios from 'axios';

const ProgressPage = () => {
  const { id } = useParams();
  const navigate = useNavigate();
  const [job, setJob] = useState(null);
  const [logs, setLogs] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const websocket = useRef(null);
  const logBoxRef = useRef(null);
  
  // Colors for the pie chart
  const COLORS = ['#2196f3', '#4caf50', '#f44336', '#ff9800'];
  
  useEffect(() => {
    const fetchJobData = async () => {
      try {
        const response = await axios.get(`/api/job/${id}`);
        setJob(response.data);
        setLoading(false);
      } catch (err) {
        setError(err.response?.data?.error || 'Failed to fetch job data');
        setLoading(false);
      }
    };
    
    fetchJobData();
    
    // Connect to WebSocket
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/${id}`;
    websocket.current = new WebSocket(wsUrl);
    
    websocket.current.onopen = () => {
      console.log('WebSocket connected');
    };
    
    websocket.current.onmessage = (event) => {
      const data = JSON.parse(event.data);
      
      if (data.type === 'log') {
        setLogs((prevLogs) => [...prevLogs, data.data]);
        // Auto scroll to bottom of logs
        if (logBoxRef.current) {
          logBoxRef.current.scrollTop = logBoxRef.current.scrollHeight;
        }
      } else {
        // Update job data
        setJob(data);
        
        // If job is complete, navigate to results page after 3 seconds
        if (data.status === 'completed') {
          setTimeout(() => {
            navigate(`/results/${id}`);
          }, 3000);
        }
      }
    };
    
    websocket.current.onerror = (error) => {
      console.error('WebSocket error:', error);
      setError('Error connecting to server. Please try refreshing the page.');
    };
    
    websocket.current.onclose = () => {
      console.log('WebSocket disconnected');
    };
    
    // Clean up
    return () => {
      if (websocket.current) {
        websocket.current.close();
      }
    };
  }, [id, navigate]);
  
  // Prepare data for pie chart
  const getPieData = () => {
    if (!job) return [];
    
    return [
      { name: 'Processing', value: job.processingCount || 0 },
      { name: 'Success', value: job.successCount || 0 },
      { name: 'Failed', value: job.failCount || 0 },
    ].filter(item => item.value > 0);
  };
  
  if (loading) {
    return (
      <Container maxWidth="md" sx={{ py: 8, textAlign: 'center' }}>
        <CircularProgress />
        <Typography variant="h6" sx={{ mt: 2 }}>
          Loading job data...
        </Typography>
      </Container>
    );
  }
  
  if (error) {
    return (
      <Container maxWidth="md" sx={{ py: 4 }}>
        <Alert severity="error">{error}</Alert>
        <Button 
          variant="contained" 
          onClick={() => navigate('/')}
          sx={{ mt: 2 }}
        >
          Back to Home
        </Button>
      </Container>
    );
  }
  
  if (!job) {
    return (
      <Container maxWidth="md" sx={{ py: 4 }}>
        <Alert severity="warning">Job not found</Alert>
        <Button 
          variant="contained" 
          onClick={() => navigate('/')}
          sx={{ mt: 2 }}
        >
          Back to Home
        </Button>
      </Container>
    );
  }
  
  const progress = job.progress * 100 || 0;
  const pieData = getPieData();
  
  return (
    <Container maxWidth="lg" sx={{ py: 4 }}>
      <GlassCard sx={{ p: 4, mb: 4 }}>
        <Typography variant="h4" gutterBottom color="primary" align="center">
          Processing Accounts
        </Typography>
        
        <Box sx={{ mb: 4 }}>
          <Grid container spacing={4}>
            <Grid item xs={12} md={6}>
              {/* Progress statistics */}
              <Box sx={{ mb: 3 }}>
                <Typography variant="body2" color="textSecondary" gutterBottom>
                  Job Status: <span style={{ fontWeight: 'bold', color: '#2196f3' }}>{job.status}</span>
                </Typography>
                
                <LinearProgress 
                  variant="determinate" 
                  value={progress} 
                  sx={{ 
                    height: 10, 
                    borderRadius: 5,
                    mb: 1,
                    backgroundColor: 'rgba(0,0,0,0.05)',
                  }} 
                />
                
                <Typography variant="body2" align="right">
                  {Math.round(progress)}%
                </Typography>
              </Box>
              
              <Grid container spacing={2}>
                <Grid item xs={4}>
                  <Paper sx={{ p: 2, textAlign: 'center', bgcolor: 'rgba(33, 150, 243, 0.1)' }}>
                    <Typography variant="h5" color="primary">{job.totalAccounts || 0}</Typography>
                    <Typography variant="body2" color="textSecondary">Total</Typography>
                  </Paper>
                </Grid>
                <Grid item xs={4}>
                  <Paper sx={{ p: 2, textAlign: 'center', bgcolor: 'rgba(76, 175, 80, 0.1)' }}>
                    <Typography variant="h5" color="success.main">{job.successCount || 0}</Typography>
                    <Typography variant="body2" color="textSecondary">Success</Typography>
                  </Paper>
                </Grid>
                <Grid item xs={4}>
                  <Paper sx={{ p: 2, textAlign: 'center', bgcolor: 'rgba(244, 67, 54, 0.1)' }}>
                    <Typography variant="h5" color="error.main">{job.failCount || 0}</Typography>
                    <Typography variant="body2" color="textSecondary">Failed</Typography>
                  </Paper>
                </Grid>
              </Grid>
            </Grid>
            
            <Grid item xs={12} md={6}>
              {/* Chart */}
              <Box sx={{ height: 200, display: 'flex', justifyContent: 'center' }}>
                {pieData.length > 0 ? (
                  <ResponsiveContainer width="100%" height="100%">
                    <PieChart>
                      <Pie
                        data={pieData}
                        cx="50%"
                        cy="50%"
                        innerRadius={60}
                        outerRadius={80}
                        fill="#8884d8"
                        paddingAngle={5}
                        dataKey="value"
                        label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
                      >
                        {pieData.map((entry, index) => (
                          <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                        ))}
                      </Pie>
                      <Tooltip formatter={(value) => [value, 'Accounts']} />
                    </PieChart>
                  </ResponsiveContainer>
                ) : (
                  <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    <Typography color="textSecondary">No data to display yet</Typography>
                  </Box>
                )}
              </Box>
            </Grid>
          </Grid>
        </Box>
        
        <Divider sx={{ my: 3 }} />
        
        {/* Log output */}
        <Typography variant="h6" gutterBottom>
          Processing Log
        </Typography>
        
        <Paper 
          elevation={0}
          sx={{ 
            p: 2, 
            height: 300, 
            bgcolor: '#1e1e1e', 
            color: '#f8f8f8',
            borderRadius: 2,
            overflowY: 'auto',
            fontFamily: 'monospace',
          }}
          ref={logBoxRef}
        >
          {logs.length > 0 ? (
            logs.map((log, index) => {
              // Determine log level color
              let color = '#f8f8f8';
              if (log.level === 'error') color = '#f44336';
              if (log.level === 'warn') color = '#ff9800';
              if (log.level === 'info') color = '#2196f3';
              if (log.level === 'debug') color = '#4caf50';
              
              return (
                <div key={index} style={{ color, marginBottom: 4 }}>
                  {log.time && <span style={{ opacity: 0.7 }}>[{new Date(log.time).toLocaleTimeString()}] </span>}
                  {log.message || JSON.stringify(log)}
                </div>
              );
            })
          ) : (
            <Typography variant="body2" sx={{ color: 'rgba(255,255,255,0.5)', fontStyle: 'italic' }}>
              Waiting for processing to start...
            </Typography>
          )}
        </Paper>
        
        <Box sx={{ mt: 3, display: 'flex', justifyContent: 'center' }}>
          <Button
            variant="contained"
            startIcon={<AssessmentIcon />}
            onClick={() => navigate(`/results/${id}`)}
            disabled={job.status !== 'completed'}
          >
            View Results
          </Button>
        </Box>
      </GlassCard>
    </Container>
  );
};

export default ProgressPage; 