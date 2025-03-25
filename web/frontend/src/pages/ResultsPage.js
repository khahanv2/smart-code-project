import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Box,
  Button,
  Container,
  Typography,
  Paper,
  Grid,
  Divider,
  CircularProgress,
  Alert,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TablePagination,
} from '@mui/material';
import DownloadIcon from '@mui/icons-material/Download';
import HomeIcon from '@mui/icons-material/Home';
import GlassCard from '../components/GlassCard';
import { PieChart, Pie, Cell, ResponsiveContainer, Tooltip, Legend } from 'recharts';
import axios from 'axios';

const ResultsPage = () => {
  const { id } = useParams();
  const navigate = useNavigate();
  const [job, setJob] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  
  // Table pagination
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(10);
  
  // Sample account data (in a real implementation, this would come from the API)
  const [accounts, setAccounts] = useState([]);
  
  // Colors for the pie chart
  const COLORS = ['#4caf50', '#f44336', '#ff9800'];
  
  useEffect(() => {
    const fetchJobData = async () => {
      try {
        const response = await axios.get(`/api/job/${id}`);
        setJob(response.data);
        
        // In a real implementation, you would fetch the accounts data as well
        // For now, we'll generate sample data
        generateSampleData(response.data);
        
        setLoading(false);
      } catch (err) {
        setError(err.response?.data?.error || 'Failed to fetch job data');
        setLoading(false);
      }
    };
    
    fetchJobData();
  }, [id]);
  
  // This is just for the demo - in a real implementation, you would get this data from the API
  const generateSampleData = (jobData) => {
    if (!jobData) return;
    
    const total = jobData.totalAccounts || 10;
    const success = jobData.successCount || 0;
    const failed = jobData.failCount || 0;
    
    const sampleAccounts = [];
    
    // Generate sample success accounts
    for (let i = 0; i < success; i++) {
      sampleAccounts.push({
        id: `acc-s-${i}`,
        username: `user_${i}`,
        status: 'success',
        balance: Math.random() * 1000,
        lastLogin: new Date().toISOString(),
      });
    }
    
    // Generate sample failed accounts
    for (let i = 0; i < failed; i++) {
      sampleAccounts.push({
        id: `acc-f-${i}`,
        username: `failed_${i}`,
        status: 'failed',
        balance: 0,
        lastLogin: null,
      });
    }
    
    setAccounts(sampleAccounts);
  };
  
  const handleChangePage = (event, newPage) => {
    setPage(newPage);
  };
  
  const handleChangeRowsPerPage = (event) => {
    setRowsPerPage(parseInt(event.target.value, 10));
    setPage(0);
  };
  
  const handleDownload = (type) => {
    if (!job) return;
    
    // Redirect to download URL
    window.location.href = `/api/download/${type}/${id}`;
  };
  
  // Prepare data for pie chart
  const getPieData = () => {
    if (!job) return [];
    
    return [
      { name: 'Success', value: job.successCount || 0 },
      { name: 'Failed', value: job.failCount || 0 },
    ].filter(item => item.value > 0);
  };
  
  if (loading) {
    return (
      <Container maxWidth="md" sx={{ py: 8, textAlign: 'center' }}>
        <CircularProgress />
        <Typography variant="h6" sx={{ mt: 2 }}>
          Loading results...
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
  
  const pieData = getPieData();
  const successRate = job.totalAccounts > 0 
    ? ((job.successCount / job.totalAccounts) * 100).toFixed(2) 
    : 0;
  
  return (
    <Container maxWidth="lg" sx={{ py: 4 }}>
      <GlassCard sx={{ p: 4, mb: 4 }}>
        <Typography variant="h4" gutterBottom color="primary" align="center">
          Processing Results
        </Typography>
        
        <Typography variant="body1" paragraph align="center">
          Job completed at {job.endTime ? new Date(job.endTime).toLocaleString() : 'N/A'}
        </Typography>
        
        <Box sx={{ mb: 4 }}>
          <Grid container spacing={4}>
            <Grid item xs={12} md={4}>
              {/* Summary */}
              <Paper sx={{ p: 3, height: '100%' }}>
                <Typography variant="h6" gutterBottom>
                  Summary
                </Typography>
                <Divider sx={{ mb: 2 }} />
                
                <Box sx={{ mb: 2 }}>
                  <Typography variant="body2" color="textSecondary">Total Accounts</Typography>
                  <Typography variant="h4" color="primary">{job.totalAccounts || 0}</Typography>
                </Box>
                
                <Box sx={{ mb: 2 }}>
                  <Typography variant="body2" color="textSecondary">Success Rate</Typography>
                  <Typography variant="h4" color="success.main">{successRate}%</Typography>
                </Box>
                
                <Box sx={{ mb: 2 }}>
                  <Typography variant="body2" color="textSecondary">Processing Time</Typography>
                  <Typography variant="h6">
                    {job.startTime && job.endTime ? (
                      `${Math.round((new Date(job.endTime) - new Date(job.startTime)) / 1000)} seconds`
                    ) : 'N/A'}
                  </Typography>
                </Box>
                
                <Box sx={{ mt: 4 }}>
                  <Button
                    variant="contained"
                    fullWidth
                    startIcon={<DownloadIcon />}
                    onClick={() => handleDownload('success')}
                    sx={{ mb: 1 }}
                  >
                    Download Success Results
                  </Button>
                  
                  <Button
                    variant="outlined"
                    fullWidth
                    startIcon={<DownloadIcon />}
                    onClick={() => handleDownload('fail')}
                    sx={{ mb: 1 }}
                  >
                    Download Failed Results
                  </Button>
                  
                  <Button
                    variant="text"
                    fullWidth
                    startIcon={<HomeIcon />}
                    onClick={() => navigate('/')}
                    sx={{ mt: 2 }}
                  >
                    Back to Home
                  </Button>
                </Box>
              </Paper>
            </Grid>
            
            <Grid item xs={12} md={8}>
              {/* Chart and Data */}
              <Grid container spacing={3}>
                <Grid item xs={12}>
                  {/* Chart */}
                  <Paper sx={{ p: 3, height: '100%' }}>
                    <Typography variant="h6" gutterBottom>
                      Account Status Distribution
                    </Typography>
                    <Divider sx={{ mb: 2 }} />
                    
                    <Box sx={{ height: 250, display: 'flex', justifyContent: 'center' }}>
                      {pieData.length > 0 ? (
                        <ResponsiveContainer width="100%" height="100%">
                          <PieChart>
                            <Pie
                              data={pieData}
                              cx="50%"
                              cy="50%"
                              labelLine={true}
                              innerRadius={70}
                              outerRadius={100}
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
                            <Legend />
                          </PieChart>
                        </ResponsiveContainer>
                      ) : (
                        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                          <Typography color="textSecondary">No data to display</Typography>
                        </Box>
                      )}
                    </Box>
                  </Paper>
                </Grid>
                
                <Grid item xs={12}>
                  {/* Data Table */}
                  <Paper sx={{ p: 0, overflow: 'hidden' }}>
                    <TableContainer sx={{ maxHeight: 400 }}>
                      <Table stickyHeader size="small">
                        <TableHead>
                          <TableRow>
                            <TableCell>Username</TableCell>
                            <TableCell>Status</TableCell>
                            <TableCell align="right">Balance</TableCell>
                            <TableCell>Last Login</TableCell>
                          </TableRow>
                        </TableHead>
                        <TableBody>
                          {accounts
                            .slice(page * rowsPerPage, page * rowsPerPage + rowsPerPage)
                            .map((account) => (
                              <TableRow 
                                key={account.id}
                                sx={{ 
                                  '&:last-child td, &:last-child th': { border: 0 },
                                  bgcolor: account.status === 'success' 
                                    ? 'rgba(76, 175, 80, 0.05)' 
                                    : 'rgba(244, 67, 54, 0.05)'
                                }}
                              >
                                <TableCell component="th" scope="row">
                                  {account.username}
                                </TableCell>
                                <TableCell>
                                  <Box
                                    component="span"
                                    sx={{
                                      px: 1,
                                      py: 0.5,
                                      borderRadius: 1,
                                      fontSize: '0.75rem',
                                      fontWeight: 'bold',
                                      color: account.status === 'success' ? '#4caf50' : '#f44336',
                                      bgcolor: account.status === 'success' 
                                        ? 'rgba(76, 175, 80, 0.1)' 
                                        : 'rgba(244, 67, 54, 0.1)',
                                    }}
                                  >
                                    {account.status.toUpperCase()}
                                  </Box>
                                </TableCell>
                                <TableCell align="right">
                                  {account.status === 'success' 
                                    ? `$${account.balance.toFixed(2)}` 
                                    : '-'
                                  }
                                </TableCell>
                                <TableCell>
                                  {account.lastLogin 
                                    ? new Date(account.lastLogin).toLocaleString() 
                                    : '-'
                                  }
                                </TableCell>
                              </TableRow>
                            ))}
                        </TableBody>
                      </Table>
                    </TableContainer>
                    <TablePagination
                      rowsPerPageOptions={[5, 10, 25, 100]}
                      component="div"
                      count={accounts.length}
                      rowsPerPage={rowsPerPage}
                      page={page}
                      onPageChange={handleChangePage}
                      onRowsPerPageChange={handleChangeRowsPerPage}
                    />
                  </Paper>
                </Grid>
              </Grid>
            </Grid>
          </Grid>
        </Box>
      </GlassCard>
    </Container>
  );
};

export default ResultsPage; 