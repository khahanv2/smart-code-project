import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
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
  Chip,
  IconButton,
  Tooltip,
} from '@mui/material';
import VisibilityIcon from '@mui/icons-material/Visibility';
import AssessmentIcon from '@mui/icons-material/Assessment';
import DeleteIcon from '@mui/icons-material/Delete';
import PlayArrowIcon from '@mui/icons-material/PlayArrow';
import GlassCard from '../components/GlassCard';
import axios from 'axios';

const JobsPage = () => {
  const navigate = useNavigate();
  const [jobs, setJobs] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    const fetchJobs = async () => {
      try {
        const response = await axios.get('/api/jobs');
        setJobs(response.data);
        setLoading(false);
      } catch (err) {
        setError(err.response?.data?.error || 'Failed to fetch jobs');
        setLoading(false);
      }
    };

    fetchJobs();
  }, []);

  const getStatusColor = (status) => {
    switch (status) {
      case 'completed':
        return 'success';
      case 'processing':
        return 'info';
      case 'pending':
        return 'warning';
      case 'failed':
        return 'error';
      default:
        return 'default';
    }
  };

  const formatDate = (dateString) => {
    if (!dateString) return 'N/A';
    return new Date(dateString).toLocaleString();
  };

  const handleViewProgress = (jobId) => {
    navigate(`/progress/${jobId}`);
  };

  const handleViewResults = (jobId) => {
    navigate(`/results/${jobId}`);
  };

  if (loading) {
    return (
      <Container maxWidth="md" sx={{ py: 8, textAlign: 'center' }}>
        <CircularProgress />
        <Typography variant="h6" sx={{ mt: 2 }}>
          Loading jobs...
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

  return (
    <Container maxWidth="lg" sx={{ py: 4 }}>
      <GlassCard sx={{ p: 4 }}>
        <Typography variant="h4" gutterBottom color="primary" align="center">
          Processing Jobs
        </Typography>
        
        <Typography variant="body1" paragraph align="center" sx={{ mb: 4 }}>
          View and manage all your batch processing jobs
        </Typography>
        
        {jobs.length === 0 ? (
          <Paper sx={{ p: 4, textAlign: 'center' }}>
            <Typography variant="h6" color="textSecondary" gutterBottom>
              No jobs found
            </Typography>
            <Typography variant="body2" color="textSecondary" paragraph>
              Upload a file to start processing
            </Typography>
            <Button 
              variant="contained" 
              onClick={() => navigate('/')}
              sx={{ mt: 2 }}
            >
              Upload File
            </Button>
          </Paper>
        ) : (
          <TableContainer component={Paper} sx={{ overflow: 'hidden' }}>
            <Table>
              <TableHead>
                <TableRow>
                  <TableCell>Job ID</TableCell>
                  <TableCell>Status</TableCell>
                  <TableCell>Total Accounts</TableCell>
                  <TableCell>Success / Failed</TableCell>
                  <TableCell>Start Time</TableCell>
                  <TableCell>Duration</TableCell>
                  <TableCell align="right">Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {jobs.map((job) => {
                  const duration = job.endTime && job.startTime
                    ? Math.round((new Date(job.endTime) - new Date(job.startTime)) / 1000)
                    : null;
                  
                  return (
                    <TableRow 
                      key={job.id}
                      sx={{ 
                        '&:last-child td, &:last-child th': { border: 0 },
                        '&:hover': { bgcolor: 'rgba(0, 0, 0, 0.04)' }
                      }}
                    >
                      <TableCell component="th" scope="row">
                        <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                          {job.id.substring(0, 8)}...
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Chip 
                          label={job.status.toUpperCase()} 
                          color={getStatusColor(job.status)}
                          size="small"
                        />
                      </TableCell>
                      <TableCell>
                        {job.totalAccounts || 0}
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          <span style={{ color: '#4caf50' }}>{job.successCount || 0}</span>
                          {' / '}
                          <span style={{ color: '#f44336' }}>{job.failCount || 0}</span>
                        </Typography>
                      </TableCell>
                      <TableCell>
                        {formatDate(job.startTime)}
                      </TableCell>
                      <TableCell>
                        {duration ? `${duration} sec` : 'In progress'}
                      </TableCell>
                      <TableCell align="right">
                        <Box sx={{ display: 'flex', justifyContent: 'flex-end' }}>
                          <Tooltip title="View Progress">
                            <IconButton 
                              color="primary"
                              onClick={() => handleViewProgress(job.id)}
                              size="small"
                            >
                              <PlayArrowIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                          
                          <Tooltip title="View Results">
                            <IconButton 
                              color="success"
                              onClick={() => handleViewResults(job.id)}
                              size="small"
                              disabled={job.status !== 'completed'}
                            >
                              <AssessmentIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                        </Box>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </TableContainer>
        )}
        
        <Box sx={{ mt: 4, textAlign: 'center' }}>
          <Button 
            variant="contained" 
            onClick={() => navigate('/')}
          >
            Start New Job
          </Button>
        </Box>
      </GlassCard>
    </Container>
  );
};

export default JobsPage; 